CREATE DATABASE IF NOT EXISTS notification_db;

USE notification_db;

CREATE TABLE IF NOT EXISTS notifications (
    id INT AUTO_INCREMENT,
    message VARCHAR(255) NOT NULL,
    ack BOOLEAN DEFAULT FALSE,
    PRIMARY KEY (id)
);

-- populate some notifications so we can test --
INSERT INTO notifications (message, ack)
VALUES 
    ('Notification 1', FALSE),
    ('Notification 2', FALSE),
    ('Notification 3', FALSE),
    ('Notification 4', FALSE),
    ('Notification 5', FALSE);
