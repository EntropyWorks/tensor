package models

import (
	database "bitbucket.pearson.com/apseng/tensor/db"
	"gopkg.in/mgo.v2/bson"
)


// Environment is the model for
// project_environment collection
type Environment struct {
	ID        bson.ObjectId `bson:"_id" json:"id"`
	Name      string        `bson:"name" json:"name" binding:"required"`
	ProjectID bson.ObjectId `bson:"project_id" json:"project_id"`
	Password  string        `bson:"password" json:"password"`
	JSON      string        `bson:"json" json:"json" binding:"required"`
}

func (env Environment) Insert() error {
	c := database.MongoDb.C("project_environments")
	return c.Insert(env)
}

func (env Environment) Update() error {
	c := database.MongoDb.C("project_environments")
	return c.UpdateId(env.ID, env)
}

func (env Environment) Remove() error {
	c := database.MongoDb.C("project_environments")
	return c.RemoveId(env.ID)
}
