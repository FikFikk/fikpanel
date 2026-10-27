-- Migrasi 011: Telemetri Mendalam WAF (GeoIP Kota, ISP, Perangkat, OS, Browser, HTTP Method)
ALTER TABLE waf_events ADD COLUMN region TEXT DEFAULT '';
ALTER TABLE waf_events ADD COLUMN city TEXT DEFAULT '';
ALTER TABLE waf_events ADD COLUMN isp TEXT DEFAULT '';
ALTER TABLE waf_events ADD COLUMN device TEXT DEFAULT '';
ALTER TABLE waf_events ADD COLUMN os TEXT DEFAULT '';
ALTER TABLE waf_events ADD COLUMN browser TEXT DEFAULT '';
ALTER TABLE waf_events ADD COLUMN method TEXT DEFAULT 'GET';
ALTER TABLE waf_events ADD COLUMN rule_name TEXT DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_waf_events_ip ON waf_events(client_ip);
CREATE INDEX IF NOT EXISTS idx_waf_events_country ON waf_events(country_code);
