-- Remove fees_manage permission from all users
UPDATE users SET permissions = array_remove(permissions, 'fees_manage')
WHERE 'fees_manage' = ANY(permissions);
