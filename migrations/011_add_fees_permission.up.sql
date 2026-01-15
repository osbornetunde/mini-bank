-- Grant fees_manage permission to existing admin users (if any)
-- This grants the permission to users who already have permissions_manage
UPDATE users SET permissions = array_append(permissions, 'fees_manage')
WHERE 'permissions_manage' = ANY(permissions)
  AND NOT ('fees_manage' = ANY(permissions));
