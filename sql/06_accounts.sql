CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY AUTO_INCREMENT NOT NULL
);

CREATE TABLE IF NOT EXISTS accounts (
	id INTEGER PRIMARY KEY AUTO_INCREMENT NOT NULL,
	username VARCHAR(128) UNIQUE NOT NULL,
	user_id INTEGER NOT NULL
);

ALTER TABLE uploads ADD user_id INTEGER NOT NULL;

INSERT INTO accounts (username) SELECT DISTINCT owner FROM uploads;

DELIMITER //
CREATE PROCEDURE migrate_accounts()
BEGIN
 DECLARE done INT DEFAULT FALSE;
 DECLARE acct_username VARCHAR(128);
 DECLARE accts CURSOR FOR SELECT username FROM accounts;
 DECLARE CONTINUE HANDLER FOR NOT FOUND SET done = TRUE;
 OPEN accts;

 read_loop: LOOP
  FETCH accts INTO acct_username;
  IF done THEN
   LEAVE read_loop;
  END IF;
  INSERT INTO users (id) VALUES (NULL);
  UPDATE accounts SET user_id = last_insert_id() WHERE username = acct_username;
  UPDATE uploads SET user_id = last_insert_id() WHERE owner = acct_username;
 END LOOP;

 CLOSE accts;
END//

DELIMITER ;

CALL migrate_accounts();

DROP PROCEDRURE migrate_accounts();

ALTER TABLE uploads ADD CONSTRAINT FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE accounts ADD CONSTRAINT FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
