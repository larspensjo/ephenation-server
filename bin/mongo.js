//
// This script will prepare the ephenation database in MongoDB. It is non-destructive,
// which means it can be executed repeatedly without destroying existing data. The following is
// not mandatory, just a recommendation.
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
db.chunkdata.ensureIndex({"x":1, "y":1, "z":1}, {unique:true})
db.chunkdata.ensureIndex({"avatarID":1}, {unique:true})

db.avatars.ensureIndex({"name":1}, {unique:true}) // Avatar name must be unique
db.avatars.ensureIndex({"owner":1}, {unique:true}) // Only one avatar per owner
db.counters.insert({_id: "avatarId", c: 0}) // A document to produce avatar IDs
// db.avatars.ensureIndex({"id":1}, {unique:true}) // _id is used for the avatar ID

// db.users.ensureIndex({"email":1}, {unique:true}) // _id is used for the user email

// db.news.ensureIndex({"id":1}, {unique:true}) // _id is used for the new ID
db.counters.insert({_id: "newsId", c: 0}) // A document to produce news IDs

