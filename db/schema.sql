CREATE TABLE IF NOT EXISTS users (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    username VARCHAR(191) NOT NULL,
    password_salt VARBINARY(32) NOT NULL,
    password_hash VARBINARY(64) NOT NULL,
    password_iter INT UNSIGNED NOT NULL,
    nickname VARCHAR(191) NOT NULL,
    profile_picture_path VARCHAR(512) NULL,
    profile_picture_mime VARCHAR(64) NULL,
    profile_picture_size BIGINT UNSIGNED NOT NULL DEFAULT 0,
    profile_picture_version BIGINT UNSIGNED NOT NULL DEFAULT 0,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY users_username_uq (username),
    KEY users_updated_at_idx (updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

