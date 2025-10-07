-- 002_add_notify_trigger.sql
-- Add PostgreSQL trigger for LISTEN/NOTIFY on trigger changes

CREATE OR REPLACE FUNCTION notify_trigger_change()
    RETURNS TRIGGER AS
$$
BEGIN
    PERFORM pg_notify('trigger_updates', row_to_json(NEW)::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_change_notify
    AFTER INSERT OR UPDATE
    ON triggers
    FOR EACH ROW
EXECUTE FUNCTION notify_trigger_change();
