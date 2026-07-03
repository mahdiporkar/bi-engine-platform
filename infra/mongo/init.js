db = db.getSiblingDB("bi_engine_platform");

db.createCollection("sessions");
db.sessions.createIndex({ expiresAt: 1 }, { expireAfterSeconds: 0 });

