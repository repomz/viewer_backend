# HTTPS for the Viewer IP address

The frontend can expose HTTPS on port 443 using a short-lived Let's Encrypt
certificate for `135.106.130.37`. The certificate is renewed automatically by
the systemd timer because IP-address certificates are valid for about six days.

The initial certificate is issued with Certbot webroot validation after the
frontend containing `/.well-known/acme-challenge/` support is running:

```sh
mkdir -p /opt/viewer/certbot/{etc,lib,www} /opt/viewer/tls
docker run --rm \
  -v /opt/viewer/certbot/etc:/etc/letsencrypt \
  -v /opt/viewer/certbot/lib:/var/lib/letsencrypt \
  -v /opt/viewer/certbot/www:/var/www/certbot \
  certbot/certbot:latest certonly \
  --webroot --webroot-path /var/www/certbot \
  --preferred-profile shortlived \
  --ip-address 135.106.130.37 \
  --non-interactive --agree-tos --register-unsafely-without-email
```

Install the renewal job:

```sh
install -o root -g root -m 0755 deploy/https/renew-viewer-ip-certificate.sh /usr/local/sbin/renew-viewer-ip-certificate
install -o root -g root -m 0644 deploy/https/viewer-tls-renew.service /etc/systemd/system/viewer-tls-renew.service
install -o root -g root -m 0644 deploy/https/viewer-tls-renew.timer /etc/systemd/system/viewer-tls-renew.timer
systemctl daemon-reload
systemctl enable --now viewer-tls-renew.timer
systemctl start viewer-tls-renew.service
```
