CREATE TABLE "log_pipelines" (
    -- Primary key for the log pipelines table
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    -- Name of the log pipeline
    name VARCHAR(100) NOT NULL,
    -- The source for input
    source VARCHAR(255) NOT NULL,
    -- Parser configurations to send to
    parsers JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);