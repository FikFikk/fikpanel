package nginx

import (
	"fmt"
	"strings"
	"time"
)

// defaultDevConfig adalah nginx.conf contoh untuk mode simulasi (non-Linux)
const defaultDevConfig = `user www-data;
worker_processes auto;
pid /run/nginx.pid;
error_log /var/log/nginx/error.log crit;
worker_rlimit_nofile 51200;

events {
    use epoll;
    worker_connections 51200;
    multi_accept on;
}

http {
    include       mime.types;
    default_type  application/octet-stream;

    server_names_hash_bucket_size 512;
    client_header_buffer_size 32k;
    large_client_header_buffers 4 32k;
    client_max_body_size 50m;
    client_body_buffer_size 512k;

    sendfile   on;
    tcp_nopush on;
    keepalive_timeout 60;
    tcp_nodelay on;

    gzip on;
    gzip_min_length 1k;
    gzip_buffers 4 16k;
    gzip_http_version 1.1;
    gzip_comp_level 5;
    gzip_types text/plain application/javascript application/json text/css application/xml;
    gzip_vary on;

    server_tokens off;
    access_log off;

    server {
        listen 127.0.0.1:80;
        server_name 127.0.0.1;

        location = /nginx_status {
            stub_status;
            allow 127.0.0.1;
            deny all;
        }
    }

    include /etc/nginx/conf.d/*.conf;
    include /etc/nginx/sites-enabled/*;
}
`

// sampleErrorLog menghasilkan baris log error contoh dengan timestamp relatif saat ini
func sampleErrorLog() string {
	now := time.Now()
	entries := []struct {
		ago   time.Duration
		level string
		msg   string
	}{
		{3 * time.Hour, "notice", "signal process started"},
		{150 * time.Minute, "warn", `conflicting server name "example.com" on 0.0.0.0:80, ignored`},
		{2 * time.Hour, "error", `open() "/var/www/html/favicon.ico" failed (2: No such file or directory), client: 10.0.0.5, server: _, request: "GET /favicon.ico HTTP/1.1"`},
		{90 * time.Minute, "error", `connect() failed (111: Connection refused) while connecting to upstream, client: 10.0.0.8, server: app.local, request: "GET /api HTTP/1.1", upstream: "http://127.0.0.1:3000/api"`},
		{45 * time.Minute, "crit", `SSL_do_handshake() failed (SSL: error:0A00006C:SSL routines::bad key share) while SSL handshaking, client: 203.0.113.9`},
		{20 * time.Minute, "warn", `an upstream response is buffered to a temporary file /var/lib/nginx/proxy/1/00/0000000001`},
		{5 * time.Minute, "notice", "signal 1 (SIGHUP) received, reconfiguring"},
	}
	var b strings.Builder
	for i, e := range entries {
		ts := now.Add(-e.ago).Format("2006/01/02 15:04:05")
		fmt.Fprintf(&b, "%s [%s] %d#%d: *%d %s\n", ts, e.level, 1024, 1025, 100+i, e.msg)
	}
	return b.String()
}
