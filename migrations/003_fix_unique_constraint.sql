-- Drop the incorrect unique constraint that causes data loss
ALTER TABLE delegations DROP CONSTRAINT IF EXISTS delegations_delegator_level_key;

-- Add operation_hash column to store the unique transaction hash
ALTER TABLE delegations ADD COLUMN IF NOT EXISTS operation_hash TEXT;

-- Create new unique constraint on operation_hash (truly unique per delegation)
ALTER TABLE delegations ADD CONSTRAINT delegations_operation_hash_key UNIQUE (operation_hash);

-- Create index for performance
CREATE INDEX IF NOT EXISTS idx_delegations_level ON delegations(level);