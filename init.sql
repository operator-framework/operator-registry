CREATE TABLE operatorbundle (
	id   INTEGER, 
	name TEXT,  
	csv TEXT, 
	bundle TEXT
);
CREATE TABLE package (
	id   INTEGER, 
	name TEXT, 
);
CREATE TABLE channel (
	name TEXT, 
	package_id INTEGER, 
	operatorbundle_id INTEGER,
	FOREIGN KEY(package_id) REFERENCES package(id),
	FOREIGN KEY(operatorbundle_id) REFERENCES operatorbundle(id)
)