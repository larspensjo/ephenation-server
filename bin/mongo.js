//
// This script will prepare the ephenation database in MongoDB. It is destructive,
// which means it will destroy any existing data!
//
// The following is not mandatory, just a recommendation.
//
// 1. Initialize the MongoDB. Proposal is as follows (from the 'mongo localhost' command):
//	use admin
//	db.addUser("admin", "somegoodpassword")
//	use ephenation
//	db.addUser("ephenation", "anothergoodpassword")
//
// 2. Restart the MongoDB server with 'mongod --auth --port port', which will enforce authentication and using port 'port'.
//
// 3. Now run this script with the command:
//
// mongo -u ephenation -p anothergoodpassword host:port/ephenation mongo.js
//
// 'host': The name of the host where the DB is, e.g. 'localhost'
// 'port': The port number used. Can be removed if default.
//

// The counters collection of documents is used to povide unique IDs.
db.counters.drop()
db.counters.insert({_id: "avatarId", c: 1}) // A document to produce avatar IDs. 0 is reserved.
db.counters.insert({_id: "newsId", c: 0}) // A document to produce news IDs

// Avatars: _id is used for the numerical avatar Id.
db.avatars.drop()
db.avatars.ensureIndex({"name":1}, {unique:true}) // Avatar name must be unique
db.avatars.ensureIndex({"email":1}, {unique:true}) // Only one avatar per owner
db.avatars.ensureIndex({"level":1}, {unique:false}) // Used for sorting
db.avatars.ensureIndex({"timeonline":1}, {unique:false}) // Used for sorting
db.avatars.ensureIndex({"tscoretotal":1}, {unique:false}) // Used for sorting

// News: _id is used for the numerical unique id.
db.news.drop()
