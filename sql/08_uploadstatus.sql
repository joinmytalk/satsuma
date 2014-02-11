ALTER TABLE uploads ADD conversion ENUM('progress', 'success', 'error') DEFAULT 'success';
