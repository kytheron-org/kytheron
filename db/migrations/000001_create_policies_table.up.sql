CREATE TABLE "policies" (
    -- Primary key for the policies table
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    -- Name of the policy
    name VARCHAR(100) NOT NULL,
    -- Relative storage path of the policy contents
    path VARCHAR(255) NOT NULL,
    -- Operational timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);