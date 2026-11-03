INSERT INTO couriers (code, name) VALUES ('recibelo', 'Recíbelo')
ON CONFLICT (code) DO NOTHING;
