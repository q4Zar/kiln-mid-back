-- Create delegations table
CREATE TABLE IF NOT EXISTS delegations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    amount TEXT NOT NULL,
    delegator TEXT NOT NULL,
    level TEXT NOT NULL,
    block_hash TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(delegator, level)
);

-- Create indexes for better query performance
CREATE INDEX idx_delegations_timestamp ON delegations(timestamp DESC);
CREATE INDEX idx_delegations_delegator ON delegations(delegator);
CREATE INDEX idx_delegations_level ON delegations(level);
CREATE INDEX idx_delegations_created_at ON delegations(created_at DESC);

-- Create metadata table for tracking indexing progress
CREATE TABLE IF NOT EXISTS indexing_metadata (
    id SERIAL PRIMARY KEY,
    last_indexed_level BIGINT NOT NULL DEFAULT 0,
    last_indexed_timestamp TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Insert initial metadata record
INSERT INTO indexing_metadata (last_indexed_level, last_indexed_timestamp)
VALUES (0, NULL)
ON CONFLICT DO NOTHING;