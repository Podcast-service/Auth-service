CREATE TABLE IF NOT EXISTS users
(
    id uuid PRIMARY KEY,
    email varchar(255) NOT NULL UNIQUE,
    password_hash text NOT NULL,
    email_verified boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS roles
(
    id uuid PRIMARY KEY,
    name varchar(50) NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS user_roles
(
    user_id uuid REFERENCES users (id) ON DELETE CASCADE,
    role_id uuid REFERENCES roles (id) ON DELETE CASCADE,
    assigned_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE IF NOT EXISTS refresh_tokens
(
    id uuid PRIMARY KEY,
    user_id uuid REFERENCES users (id) ON DELETE CASCADE,
    token_hash text NOT NULL UNIQUE,
    device_name varchar(255),
    ip_address inet,
    user_agent text,
    expires_at timestamptz NOT NULL,
    revoked boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS email_verification_tokens
(
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  code varchar(6) NOT NULL,
  expires_at timestamptz NOT NULL,
  used boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS password_reset_tokens
(
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code varchar(6) NOT NULL,
    expires_at timestamptz NOT NULL,
    used boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id    ON refresh_tokens (user_id);
CREATE INDEX IF NOT EXISTS idx_evt_user_id    ON email_verification_tokens (user_id);
CREATE INDEX IF NOT EXISTS idx_prt_user_id    ON password_reset_tokens (user_id);
CREATE INDEX IF NOT EXISTS idx_evt_user_code ON email_verification_tokens (user_id, code);
CREATE INDEX IF NOT EXISTS idx_prt_user_code ON password_reset_tokens (user_id, code);

INSERT INTO roles (id, name) VALUES
                                 ('00000000-0000-0000-0000-000000000001', 'user'),
                                 ('00000000-0000-0000-0000-000000000002', 'admin');

-- оставляем только последние 3 валидных кода верификации email
CREATE OR REPLACE FUNCTION limit_email_verify_tokens()
RETURNS TRIGGER AS $$
BEGIN
DELETE FROM email_verification_tokens
WHERE user_id = NEW.user_id
  AND id NOT IN (
    SELECT id
    FROM email_verification_tokens
    WHERE user_id = NEW.user_id
    ORDER BY created_at DESC
    LIMIT 3
    );
RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_limit_email_verify_tokens
    AFTER INSERT ON email_verification_tokens
    FOR EACH ROW
    EXECUTE FUNCTION limit_email_verify_tokens();

-- оставляем только последние 3 валидных кода сброса пароля
CREATE OR REPLACE FUNCTION limit_password_reset_tokens()
RETURNS TRIGGER AS $$
BEGIN
DELETE FROM password_reset_tokens
WHERE user_id = NEW.user_id
  AND id NOT IN (
    SELECT id
    FROM password_reset_tokens
    WHERE user_id = NEW.user_id
    ORDER BY created_at DESC
    LIMIT 3
    );
RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_limit_password_reset_tokens
    AFTER INSERT ON password_reset_tokens
    FOR EACH ROW
    EXECUTE FUNCTION limit_password_reset_tokens();

-- удаляем все коды верификации после подтверждения email
CREATE OR REPLACE FUNCTION cleanup_email_verify_tokens()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.email_verified = false AND NEW.email_verified = true THEN
DELETE FROM email_verification_tokens
WHERE user_id = NEW.id;
END IF;
RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_cleanup_email_verify_tokens
    AFTER UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION cleanup_email_verify_tokens();

-- удаляем все коды сброса пароля после смены пароля
CREATE OR REPLACE FUNCTION cleanup_password_reset_tokens()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.password_hash != NEW.password_hash THEN
DELETE FROM password_reset_tokens
WHERE user_id = NEW.id;
END IF;
RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_cleanup_password_reset_tokens
    AFTER UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION cleanup_password_reset_tokens();
