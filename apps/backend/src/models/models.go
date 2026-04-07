package models
import "time"

type User struct { ID int; Email string; Name string; Role string; CreatedAt time.Time }
type Team struct { ID int; Name string; OwnerID int; CreatedAt time.Time }
type Repository struct { ID int; TeamID int; Name string; URL string; EncryptedCredential string `json:"-"`; CreatedAt time.Time }
type Annotation struct { ID int; RepositoryID int; Type string; Payload string; AuthorID int; CreatedAt time.Time }
