DELETE FROM user_companies WHERE user_id IN (SELECT id FROM users WHERE email = 'cliente2@nexus.local');
DELETE FROM users WHERE email = 'cliente2@nexus.local';
