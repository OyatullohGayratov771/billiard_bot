#!/bin/bash
# SSL sertifikat olish (Let's Encrypt)
# Ishlatish: bash nginx/get_cert.sh sizning-domen.uz

set -e

DOMAIN="${1:?Domen nomini kiriting: bash get_cert.sh domen.uz}"
CERT_DIR="./nginx/certs"

mkdir -p "$CERT_DIR"

# certbot o'rnatilganmi?
if ! command -v certbot &>/dev/null; then
    echo "certbot o'rnatilmoqda..."
    apt-get update -qq && apt-get install -y certbot
fi

echo "📜 $DOMAIN uchun sertifikat olinmoqda..."

certbot certonly \
    --standalone \
    --non-interactive \
    --agree-tos \
    --email "admin@${DOMAIN}" \
    -d "$DOMAIN" \
    --cert-path   "$CERT_DIR/fullchain.pem" \
    --key-path    "$CERT_DIR/privkey.pem"

# Yoki Let's Encrypt standart joylashuvidan nusxa
if [ ! -f "$CERT_DIR/fullchain.pem" ]; then
    cp "/etc/letsencrypt/live/$DOMAIN/fullchain.pem" "$CERT_DIR/fullchain.pem"
    cp "/etc/letsencrypt/live/$DOMAIN/privkey.pem"   "$CERT_DIR/privkey.pem"
fi

chmod 644 "$CERT_DIR/fullchain.pem"
chmod 600 "$CERT_DIR/privkey.pem"

echo "✅ Sertifikat tayyor: $CERT_DIR/"
echo "Nginx ni qayta ishga tushiring: docker-compose restart nginx"
