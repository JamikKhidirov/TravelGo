# Деплой GoTravel на VPS

Инструкция для Ubuntu 22.04/24.04 и любого VPS с Docker.

## 1. Требования

- VPS 2 CPU / 2 GB RAM / 20 GB SSD
- Ubuntu 22.04 или 24.04
- домен, направленный A-записью на IP сервера
- открытые порты `22`, `80`, `443`

## 2. Установка Docker

```bash
sudo apt update && sudo apt upgrade -y
sudo apt install -y ca-certificates curl git ufw

curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker $USER
newgrp docker

docker --version
docker compose version
```

## 3. Firewall

```bash
sudo ufw allow OpenSSH
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
sudo ufw status
```

Порт `8080` наружу не открывайте — доступ будет через reverse proxy.

## 4. Клонирование проекта

```bash
git clone https://github.com/JamikKhidirov/TravelGo.git
cd TravelGo
```

Если репозиторий уже содержит вложенную папку:

```bash
cd go-travel
```

## 5. Настройка

Отредактируйте `configs/config.yaml`:

```yaml
app:
  env: production
  port: 8080
db:
  host: postgres
  port: 5432
  user: travel
  password: change-me
  name: travel_db
  sslmode: disable
redis:
  addr: redis:6379
```

Пароль PostgreSQL лучше задать через `.env` и `docker compose`, а не хранить в git.

Пример `.env`:

```env
DB_PASSWORD=strong-password
FCM_SERVER_KEY=your-firebase-server-key
```

`FCM_SERVER_KEY` нужен для реальных push-уведомлений на iOS и Android через Firebase Cloud Messaging. Без него сервис работает с `log`-провайдером.

## 6. Запуск

```bash
docker compose up -d --build
docker compose ps
docker compose logs -f app
```

Проверка:

```bash
curl http://127.0.0.1:8080/health
curl -X POST http://127.0.0.1:8080/api/v1/sync
curl "http://127.0.0.1:8080/api/v1/tickets?page_size=5"
```

## 7. Reverse proxy и HTTPS

### Вариант A: Caddy (просто)

```bash
sudo apt install -y caddy
sudo nano /etc/caddy/Caddyfile
```

```caddy
travel.example.com {
    encode gzip
    reverse_proxy 127.0.0.1:8080
}
```

```bash
sudo systemctl restart caddy
sudo systemctl enable caddy
```

Caddy автоматически получит TLS-сертификат и поддерживает WebSocket.

### Вариант B: Nginx

```bash
sudo apt install -y nginx certbot python3-certbot-nginx
sudo nano /etc/nginx/sites-available/gotravel
```

```nginx
server {
    listen 80;
    server_name travel.example.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /ws {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_read_timeout 3600s;
    }
}
```

```bash
sudo ln -s /etc/nginx/sites-available/gotravel /etc/nginx/sites-enabled/gotravel
sudo nginx -t
sudo systemctl reload nginx
sudo certbot --nginx -d travel.example.com
```

После TLS:

- API: `https://travel.example.com`
- Swagger: `https://travel.example.com/swagger/index.html`
- Admin: `https://travel.example.com/admin`
- WebSocket: `wss://travel.example.com/ws?room=global`

## 8. Обновление

```bash
git pull origin master
docker compose up -d --build
docker image prune -f
```

## 9. Резервное копирование PostgreSQL

```bash
docker exec -t travel-postgres pg_dump -U travel travel_db > backup_$(date +%F).sql
```

Восстановление:

```bash
cat backup_2026-10-23.sql | docker exec -i travel-postgres psql -U travel travel_db
```

Добавьте backup в cron:

```cron
0 3 * * * cd /home/ubuntu/TravelGo && docker exec -t travel-postgres pg_dump -U travel travel_db > backups/db_$(date +\%F).sql
```

## 10. Запуск без Docker (бинарник)

Для Linux используйте релизный файл `go-travel-server-linux-amd64`:

```bash
chmod +x go-travel-server-linux-amd64
./go-travel-server-linux-amd64 -config ./configs/config.yaml
```

Для Windows используйте `go-travel-server-windows-amd64.exe`:

```powershell
.\go-travel-server-windows-amd64.exe -config .\configs\config.yaml
```

Бинарники публикуются в GitHub Releases при пуше тега `v*`.

## 11. Systemd (опционально)

```ini
[Unit]
Description=GoTravel API
After=network.target

[Service]
WorkingDirectory=/opt/gotravel
ExecStart=/opt/gotravel/go-travel-server-linux-amd64 -config /opt/gotravel/configs/config.yaml
Restart=always
RestartSec=5
Environment=FCM_SERVER_KEY=your-key

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now gotravel
sudo journalctl -u gotravel -f
```

## 12. Диагностика

```bash
docker compose ps
docker compose logs -f app
docker compose logs -f postgres
curl -I https://travel.example.com/health
```

Если WebSocket не подключается, проверьте заголовки `Upgrade`/`Connection` в Nginx и используйте `wss://` при HTTPS.
