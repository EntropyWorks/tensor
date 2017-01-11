package metadata

import (
	"github.com/gamunu/tensor/db"
	"github.com/gamunu/tensor/models"
	log "github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
)

// Create a new organization
func GroupMetadata(grp *models.Group) {

	ID := grp.ID.Hex()
	grp.Type = "group"
	grp.Url = "/v1/group/" + ID + "/"
	grp.Related = gin.H{
		"created_by":         "/v1/users/" + grp.CreatedByID.Hex() + "/",
		"job_host_summaries": "/v1/groups/" + grp.CreatedByID.Hex() + "job_host_summaries/",
		"variable_data":      "/v1/groups/" + grp.CreatedByID.Hex() + "/variable_data/",
		"job_events":         "/v1/groups/" + grp.CreatedByID.Hex() + "/job_events/",
		"potential_children": "/v1/groups/" + grp.CreatedByID.Hex() + "/potential_children/",
		"ad_hoc_commands":    "/v1/groups/" + grp.CreatedByID.Hex() + "/ad_hoc_commands/",
		"all_hosts":          "/v1/groups/" + grp.CreatedByID.Hex() + "/all_hosts/",
		"activity_stream":    "/v1/groups/" + grp.CreatedByID.Hex() + "/activity_stream/",
		"hosts":              "/v1/groups/" + grp.CreatedByID.Hex() + "/hosts/",
		"children":           "/v1/groups/" + grp.CreatedByID.Hex() + "/children/",
		"inventory_sources":  "/v1/groups/" + grp.CreatedByID.Hex() + "/inventory_sources/",
		"inventory":          "/v1/inventories/" + grp.InventoryID.Hex() + "/",
		"inventory_source":   "/v1/inventory_sources/emptyid/",
	}

	groupSummary(grp)
}

func groupSummary(grp *models.Group) {

	var modified models.User
	var created models.User
	var inv models.Inventory

	summary := gin.H{
		"inventory": nil,
		"inventory_source": gin.H{
			"source": "",
			"status": "none",
		},
		"modified_by": nil,
		"created_by":  nil,
	}

	if err := db.Users().FindId(grp.CreatedByID).One(&created); err != nil {
		log.WithFields(log.Fields{
			"User ID":  grp.CreatedByID.Hex(),
			"Group":    grp.Name,
			"Group ID": grp.ID.Hex(),
		}).Errorln("Error while getting created by User")
	} else {
		summary["created_by"] = gin.H{
			"id":         created.ID.Hex(),
			"username":   created.Username,
			"first_name": created.FirstName,
			"last_name":  created.LastName,
		}
	}

	if err := db.Users().FindId(grp.ModifiedByID).One(&modified); err != nil {
		log.WithFields(log.Fields{
			"User ID":  grp.ModifiedByID.Hex(),
			"Group":    grp.Name,
			"Group ID": grp.ID.Hex(),
		}).Errorln("Error while getting modified by User")
	} else {
		summary["modified_by"] = gin.H{
			"id":         created.ID.Hex(),
			"username":   created.Username,
			"first_name": created.FirstName,
			"last_name":  created.LastName,
		}
	}

	if err := db.Inventories().FindId(grp.InventoryID).One(&inv); err != nil {
		log.WithFields(log.Fields{
			"Inventory ID": grp.InventoryID.Hex(),
			"Group":        grp.Name,
			"Group ID":     grp.ID.Hex(),
		}).Errorln("Error while getting Inventory")
	} else {
		summary["inventory"] = gin.H{
			"id":                              inv.ID,
			"name":                            inv.Name,
			"description":                     inv.Description,
			"has_active_failures":             inv.HasActiveFailures,
			"total_hosts":                     inv.TotalHosts,
			"hosts_with_active_failures":      inv.HostsWithActiveFailures,
			"total_groups":                    inv.TotalGroups,
			"groups_with_active_failures":     inv.GroupsWithActiveFailures,
			"has_inventory_sources":           inv.HasInventorySources,
			"total_inventory_sources":         inv.TotalInventorySources,
			"inventory_sources_with_failures": inv.InventorySourcesWithFailures,
		}
	}

	grp.Summary = summary
}
