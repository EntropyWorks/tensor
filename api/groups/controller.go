package groups

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"bitbucket.pearson.com/apseng/tensor/api/helpers"
	"bitbucket.pearson.com/apseng/tensor/api/metadata"
	"bitbucket.pearson.com/apseng/tensor/db"
	"bitbucket.pearson.com/apseng/tensor/models"
	"bitbucket.pearson.com/apseng/tensor/util"
	log "github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"gopkg.in/mgo.v2/bson"
)

// Keys for group releated items stored in the Gin Context
const (
	CTXGroup   = "group"
	CTXUser    = "user"
	CTXGroupID = "group_id"
)

// Middleware generates a middleware handler function that works inside of a Gin request.
// This function takes host_id parameter from the Gin Context and fetches host data from the database
// it will set host data under key host in the Gin Context.
func Middleware(c *gin.Context) {

	ID, err := util.GetIdParam(CTXGroupID, c)

	if err != nil {
		log.WithFields(log.Fields{
			"Group ID": ID,
			"Error":    err.Error(),
		}).Errorln("Error while getting Group ID url parameter")
		c.JSON(http.StatusNotFound, models.Error{
			Code:     http.StatusNotFound,
			Messages: []string{"Not Found"},
		})
		c.Abort()
		return
	}

	var group models.Group
	err = db.Groups().FindId(bson.ObjectIdHex(ID)).One(&group)

	if err != nil {
		log.WithFields(log.Fields{
			"Group ID": ID,
			"Error":    err.Error(),
		}).Errorln("Error while retriving Group form the database")
		c.JSON(http.StatusNotFound, models.Error{
			Code:     http.StatusNotFound,
			Messages: []string{"Not Found"},
		})
		c.Abort()
		return
	}

	c.Set(CTXGroup, group)
	c.Next()
}

// GetGroup is a Gin handler function which returns the host as a JSON object.
func GetGroup(c *gin.Context) {
	group := c.MustGet(CTXGroup).(models.Group)

	metadata.GroupMetadata(&group)
	// send response with JSON rendered data
	c.JSON(http.StatusOK, group)
}

// GetGroups is a Gin handler function which returns list of Groups
// This takes lookup parameters and order parameters to filder and sort output data.
func GetGroups(c *gin.Context) {
	user := c.MustGet(CTXUser).(models.User)

	parser := util.NewQueryParser(c)
	match := bson.M{}
	match = parser.Match([]string{"source", "has_active_failures"}, match)
	match = parser.Lookups([]string{"name", "description"}, match)

	query := db.Groups().Find(match) // prepare the query
	// set sort value to the query based on request parameters
	order := parser.OrderBy()
	if order != "" {
		query.Sort(order)
	}

	log.WithFields(log.Fields{
		"Query": query,
	}).Debugln("Parsed query")

	var groups []models.Group
	// new mongodb iterator
	iter := query.Iter()
	// loop through each result and modify for our needs
	var tmpGroup models.Group
	// iterate over all and only get valid objects
	for iter.Next(&tmpGroup) {
		metadata.GroupMetadata(&tmpGroup)
		// good to go add to list
		groups = append(groups, tmpGroup)
	}
	if err := iter.Close(); err != nil {
		log.WithFields(log.Fields{
			"User ID":  user.ID.Hex(),
			"Group ID": tmpGroup.ID.Hex(),
		}).Debugln("User does not have read permissions")
		c.JSON(http.StatusInternalServerError, models.Error{
			Code:     http.StatusInternalServerError,
			Messages: []string{"Error while getting Group"},
		})
		return
	}

	count := len(groups)
	pgi := util.NewPagination(c, count)
	//if page is incorrect return 404
	if pgi.HasPage() {
		log.WithFields(log.Fields{
			"Page number": pgi.Page(),
		}).Debugln("Group page does not exist")
		c.JSON(http.StatusNotFound, gin.H{"detail": "Invalid page " + strconv.Itoa(pgi.Page()) + ": That page contains no results."})
		return
	}

	log.WithFields(log.Fields{
		"Count":    count,
		"Next":     pgi.NextPage(),
		"Previous": pgi.PreviousPage(),
		"Skip":     pgi.Skip(),
		"Limit":    pgi.Limit(),
	}).Debugln("Response info")

	// send response with JSON rendered data
	c.JSON(http.StatusOK, models.Response{
		Count:    count,
		Next:     pgi.NextPage(),
		Previous: pgi.PreviousPage(),
		Results:  groups[pgi.Skip():pgi.End()],
	})
}

// AddGroup is a Gin handler function which creates a new group using request payload.
// This accepts Group model.
func AddGroup(c *gin.Context) {
	var req models.Group
	// get user from the gin.Context
	user := c.MustGet(CTXUser).(models.User)

	err := binding.JSON.Bind(c.Request, &req)
	if err != nil {
		log.WithFields(log.Fields{
			"Error": err.Error(),
		}).Errorln("Invlid JSON request")
		c.JSON(http.StatusBadRequest, models.Error{
			Code:     http.StatusBadRequest,
			Messages: util.GetValidationErrors(err),
		})
		return
	}

	// if the group exist in the collection it is not unique
	if helpers.IsNotUniqueGroup(req.Name, req.InventoryID) {
		c.JSON(http.StatusBadRequest, models.Error{
			Code:     http.StatusBadRequest,
			Messages: []string{"Group with this Name and Inventory already exists."},
		})
		return
	}

	// check whether the inventory exist or not
	if !helpers.InventoryExist(req.InventoryID) {
		c.JSON(http.StatusBadRequest, models.Error{
			Code:     http.StatusBadRequest,
			Messages: []string{"Inventory does not exists."},
		})
		return
	}

	// check whether the group exist or not
	if req.ParentGroupID != nil {
		if !helpers.ParentGroupExist(*req.ParentGroupID) {
			c.JSON(http.StatusBadRequest, models.Error{
				Code:     http.StatusBadRequest,
				Messages: []string{"Parent Group does not exists."},
			})
			return
		}
	}

	// create new object to omit unnecessary fields
	req.ID = bson.NewObjectId()
	req.Created = time.Now()
	req.Modified = time.Now()
	req.CreatedByID = user.ID
	req.ModifiedByID = user.ID

	if err = db.Groups().Insert(req); err != nil {
		log.WithFields(log.Fields{
			"Group ID": req.ID.Hex(),
			"Error":    err.Error(),
		}).Errorln("Error while creating Group")
		c.JSON(http.StatusInternalServerError, models.Error{
			Code:     http.StatusInternalServerError,
			Messages: []string{"Error while creating Group"},
		})
		return
	}

	// add new activity to activity stream
	if err := db.ActivityStream().Insert(models.Activity{
		ID:          bson.NewObjectId(),
		ActorID:     user.ID,
		Type:        CTXGroup,
		ObjectID:    req.ID,
		Description: "Group " + req.Name + " created",
		Created:     time.Now(),
	}); err != nil {
		log.WithFields(log.Fields{
			"Error": err.Error(),
		}).Errorln("Failed to add new Activity")
	}

	metadata.GroupMetadata(&req)

	// send response with JSON rendered data
	c.JSON(http.StatusCreated, req)
}

// UpdateGroup is a handler function which updates a group using request payload.
// This replaces all the fields in the database. empty "" fields and
// unspecified fields will be removed from the database object.
func UpdateGroup(c *gin.Context) {
	// get Group from the gin.Context
	group := c.MustGet(CTXGroup).(models.Group)
	// get user from the gin.Context
	user := c.MustGet(CTXUser).(models.User)

	var req models.Group
	if err := binding.JSON.Bind(c.Request, &req); err != nil {
		// Return 400 if request has bad JSON format
		c.JSON(http.StatusBadRequest, models.Error{
			Code:     http.StatusBadRequest,
			Messages: util.GetValidationErrors(err),
		})
		return
	}

	// check whether the inventory exist or not
	if !helpers.InventoryExist(req.InventoryID) {
		c.JSON(http.StatusBadRequest, models.Error{
			Code:     http.StatusBadRequest,
			Messages: []string{"Inventory does not exists."},
		})
		return
	}

	if req.Name != group.Name {
		// if the group exist in the collection it is not unique
		if helpers.IsNotUniqueGroup(req.Name, req.InventoryID) {
			c.JSON(http.StatusBadRequest, models.Error{
				Code:     http.StatusBadRequest,
				Messages: []string{"Group with this Name and Inventory already exists."},
			})
			return
		}
	}

	// check whether the group exist or not
	if req.ParentGroupID != nil {
		if !helpers.ParentGroupExist(*req.ParentGroupID) {
			c.JSON(http.StatusBadRequest, models.Error{
				Code:     http.StatusBadRequest,
				Messages: []string{"Parent Group does not exists."},
			})
			return
		}
	}

	group.Name = strings.Trim(req.Name, " ")
	group.Description = strings.Trim(req.Description, " ")
	group.Variables = req.Variables
	group.InventoryID = req.InventoryID
	group.ParentGroupID = req.ParentGroupID
	group.ParentGroupID = req.ParentGroupID
	group.Modified = time.Now()
	group.ModifiedByID = user.ID

	// update object
	if err := db.Groups().UpdateId(group.ID, group); err != nil {
		log.Errorln("Error while updating Group:", err)
		c.JSON(http.StatusInternalServerError, models.Error{
			Code:     http.StatusInternalServerError,
			Messages: []string{"Error while updating Group"},
		})
		return
	}

	// add new activity to activity stream
	if err := db.ActivityStream().Insert(models.Activity{
		ID:          bson.NewObjectId(),
		ActorID:     user.ID,
		Type:        CTXGroup,
		ObjectID:    group.ID,
		Description: "Group " + group.Name + " updated",
		Created:     time.Now(),
	}); err != nil {
		log.WithFields(log.Fields{
			"Error": err.Error(),
		}).Errorln("Failed to add new Activity")
	}

	// set `related` and `summary` feilds
	metadata.GroupMetadata(&group)

	// send response with JSON rendered data
	c.JSON(http.StatusOK, group)
}

// PatchGroup is a Gin handler function which partially updates a group using request payload.
// This replaces specified fields in the database, empty "" fields will be
// removed from the database object. Unspecified fields will be ignored.
func PatchGroup(c *gin.Context) {
	// get Group from the gin.Context
	group := c.MustGet(CTXGroup).(models.Group)
	// get user from the gin.Context
	user := c.MustGet(CTXUser).(models.User)

	var req models.PatchGroup
	if err := binding.JSON.Bind(c.Request, &req); err != nil {
		// Return 400 if request has bad JSON format
		c.JSON(http.StatusBadRequest, models.Error{
			Code:     http.StatusBadRequest,
			Messages: util.GetValidationErrors(err),
		})
		return
	}

	// check whether the inventory exist or not
	if req.InventoryID != nil {
		if !helpers.InventoryExist(*req.InventoryID) {
			c.JSON(http.StatusBadRequest, models.Error{
				Code:     http.StatusBadRequest,
				Messages: []string{"Inventory does not exists."},
			})
			return
		}
	}

	// since this is a patch request if the name specified check the
	// inventory name is unique
	if req.Name != nil && *req.Name != group.Name {
		objID := group.InventoryID
		// if inventory id specified use it otherwise use
		// old inventory id
		if req.InventoryID != nil {
			objID = *req.InventoryID
		}
		// if the group exist in the collection it is not unique
		if helpers.IsNotUniqueGroup(*req.Name, objID) {
			c.JSON(http.StatusBadRequest, models.Error{
				Code:     http.StatusBadRequest,
				Messages: []string{"Group with this Name and Inventory already exists."},
			})
			return
		}
	}

	// check whether the group exist or not
	if req.ParentGroupID != nil {
		if !helpers.ParentGroupExist(*req.ParentGroupID) {
			c.JSON(http.StatusBadRequest, models.Error{
				Code:     http.StatusBadRequest,
				Messages: []string{"Parent Group does not exists."},
			})
			return
		}
	}

	if req.Name != nil {
		group.Name = strings.Trim(*req.Name, " ")
	}

	if req.Description != nil {
		group.Description = strings.Trim(*req.Description, " ")
	}

	if req.Variables != nil {
		group.Variables = *req.Variables
	}

	if req.InventoryID != nil {
		group.InventoryID = *req.InventoryID
	}

	if req.ParentGroupID != nil {
		// if empty string then make the credential null
		if len(*req.ParentGroupID) == 12 {
			group.ParentGroupID = req.ParentGroupID
		} else {
			group.ParentGroupID = nil
		}
	}

	group.Modified = time.Now()
	group.ModifiedByID = user.ID

	// update object
	if err := db.Hosts().UpdateId(group.ID, group); err != nil {
		log.Errorln("Error while updating Group:", err)
		c.JSON(http.StatusInternalServerError, models.Error{
			Code:     http.StatusInternalServerError,
			Messages: []string{"Error while updating Group"},
		})
		return
	}

	// add new activity to activity stream
	if err := db.ActivityStream().Insert(models.Activity{
		ID:          bson.NewObjectId(),
		ActorID:     user.ID,
		Type:        CTXGroup,
		ObjectID:    group.ID,
		Description: "Group " + group.Name + " updated",
		Created:     time.Now(),
	}); err != nil {
		log.WithFields(log.Fields{
			"Error": err.Error(),
		}).Errorln("Failed to add new Activity")
	}

	// set `related` and `summary` fields
	metadata.GroupMetadata(&group)

	// send response with JSON rendered data
	c.JSON(http.StatusOK, group)
}

// RemoveGroup is a Gin handler function which removes a group object from the database
func RemoveGroup(c *gin.Context) {
	// get Group from the gin.Context
	group := c.MustGet(CTXGroup).(models.Group)
	// get user from the gin.Context
	user := c.MustGet(CTXUser).(models.User)

	var childgroups []models.Group

	//find the group and all child groups
	query := bson.M{
		"$or": []bson.M{
			{"parent_group_id": group.ID},
			{"_id": group.ID},
		},
	}
	err := db.Groups().Find(query).Select(bson.M{"_id": 1}).All(&childgroups)
	if err != nil {
		log.Errorln("Error while getting child Groups:", err)
		c.JSON(http.StatusInternalServerError, models.Error{
			Code:     http.StatusInternalServerError,
			Messages: []string{"Error while removing Group"},
		})
		return
	}

	// get group ids
	var ids []bson.ObjectId

	for _, v := range childgroups {
		ids = append(ids, v.ID)
	}

	//remove hosts that has group ids of group and child groups
	changes, err := db.Hosts().RemoveAll(bson.M{"group_id": bson.M{"$in": ids}})
	if err != nil {
		log.Errorln("Error while removing Group Hosts:", err)
		c.JSON(http.StatusInternalServerError, models.Error{
			Code:     http.StatusInternalServerError,
			Messages: []string{"Error while removing Group Hosts"},
		})
		return
	}
	log.Infoln("Hosts remove info:", changes.Removed)

	// remove groups from the collection
	changes, err = db.Groups().RemoveAll(query)
	if err != nil {
		log.Errorln("Error while removing Group:", err)
		c.JSON(http.StatusInternalServerError, models.Error{
			Code:     http.StatusInternalServerError,
			Messages: []string{"Error while removing Group"},
		})
		return
	}

	log.WithFields(log.Fields{
		"Removed": changes.Removed,
	}).Infoln("Groups remove info")

	// add new activity to activity stream
	if err := db.ActivityStream().Insert(models.Activity{
		ID:          bson.NewObjectId(),
		ActorID:     user.ID,
		Type:        CTXGroup,
		ObjectID:    group.ID,
		Description: "Group " + group.Name + " deleted",
		Created:     time.Now(),
	}); err != nil {
		log.WithFields(log.Fields{
			"Error": err.Error(),
		}).Errorln("Failed to add new Activity")
	}

	// abort with 204 status code
	c.AbortWithStatus(http.StatusNoContent)
}

// VariableData is Gin handler function which returns host group variables
func VariableData(c *gin.Context) {
	group := c.MustGet(CTXGroup).(models.Group)

	variables := gin.H{}

	if err := json.Unmarshal([]byte(group.Variables), &variables); err != nil {
		log.Errorln("Error while getting Group variables")
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": []string{"Error while getting Group variables"},
		})
		return
	}

	c.JSON(http.StatusOK, variables)
}