# Подготовка Ubuntu-сервера и запуск Viewer

Инструкция предназначена для нового сервера с Ubuntu Server 24.04 LTS
`amd64`. Ubuntu 22.04 LTS также подходит. На сервере запускаются:

- PostgreSQL;
- одноразовый контейнер миграций Goose;
- Viewer Backend;
- Orthanc PACS;
- OHIF Viewer.

Hospital Agent на сервер не устанавливается. Он работает на больничном
Windows-компьютере через `pythonw` и подключается к backend и PACS по сети.

Официальные инструкции, на которых основана установка:

- [Install Docker Engine on Ubuntu](https://docs.docker.com/engine/install/ubuntu/);
- [Linux post-installation steps](https://docs.docker.com/engine/install/linux-postinstall/);
- [Docker packet filtering and firewalls](https://docs.docker.com/engine/network/packet-filtering-firewalls/).

## Автоматическая подготовка нового сервера

Шаги 2–5 можно выполнить скриптом
`scripts/prepare_ubuntu_server.sh`. Скрипт:

- поддерживает Ubuntu Server 22.04/24.04 `amd64`;
- устанавливает системные обновления и базовые утилиты;
- удаляет конфликтующие Docker-пакеты;
- подключает официальный APT-репозиторий Docker;
- устанавливает Docker Engine, Buildx и Compose v2;
- включает Docker в автозагрузку;
- ограничивает размер контейнерных логов;
- добавляет SSH-пользователя в группу `docker`;
- создаёт `/opt/viewer`;
- не клонирует репозиторий и не запускает приложение.

Скопируйте скрипт с рабочего компьютера на новый сервер:

```bash
scp scripts/prepare_ubuntu_server.sh USER@SERVER_IP:/tmp/
```

Запустите на сервере:

```bash
ssh USER@SERVER_IP
chmod +x /tmp/prepare_ubuntu_server.sh
sudo /tmp/prepare_ubuntu_server.sh
```

Дополнительные параметры передаются через переменные окружения:

```bash
sudo SERVER_TIMEZONE=Asia/Tomsk \
  INSTALL_DIR=/opt/viewer \
  DOCKER_USER="$USER" \
  UPGRADE_SYSTEM=1 \
  /tmp/prepare_ubuntu_server.sh
```

Если скрипт сообщает о необходимости перезагрузки:

```bash
sudo reboot
```

После выполнения закройте SSH-сеанс и подключитесь снова, чтобы применилось
членство в группе `docker`. Затем переходите к разделу
«Сетевой доступ и firewall» и клонированию проекта.

Скрипт намеренно не меняет статический IP, firewall или Security Group:
универсальное автоматическое изменение этих настроек может заблокировать SSH
и требует знания больничной сетевой схемы.

## 1. Требования к серверу

Рекомендуемый минимум для тестового сервера:

- 64-битная Ubuntu Server 24.04 LTS;
- архитектура `amd64`/`x86_64`;
- 2 CPU;
- 4 ГБ RAM;
- системный диск от 50 ГБ;
- отдельное хранилище подходящего размера для DICOM;
- статический IP-адрес или постоянное DNS-имя;
- доступ в интернет во время установки и получения образов;
- пользователь с правом `sudo`;
- доступ по SSH.

Orthanc в текущей конфигурации не ограничивает размер DICOM-хранилища.
Свободное место необходимо контролировать отдельно.

Проверка ОС, архитектуры, памяти и диска:

```bash
cat /etc/os-release
dpkg --print-architecture
uname -m
free -h
df -h
```

Ожидаемая архитектура:

```text
amd64
x86_64
```

Если сервер имеет `arm64`, не продолжайте установку без отдельной проверки
совместимости образов Orthanc и старого OHIF.

## 2. Первичная подготовка Ubuntu

Подключитесь к серверу:

```bash
ssh USER@SERVER_IP
```

Обновите систему и установите необходимые утилиты:

```bash
sudo apt update
sudo apt full-upgrade -y
sudo apt install -y ca-certificates curl git jq openssl
```

Если обновилось ядро, перезагрузите сервер и подключитесь снова:

```bash
sudo reboot
```

Установите часовой пояс:

```bash
sudo timedatectl set-timezone Asia/Tomsk
timedatectl status
```

Проверьте, что сервер имеет статический адрес в больничной сети. Дальше этот
адрес обозначается как `SERVER_IP`.

## 3. Удаление конфликтующих Docker-пакетов

Этот шаг не удаляет каталог `/var/lib/docker`, если Docker уже использовался.
На совершенно новом сервере данных Docker ещё нет.

```bash
for pkg in docker.io docker-compose docker-compose-v2 docker-doc podman-docker containerd runc; do
  sudo apt remove -y "$pkg"
done
```

Сообщения о том, что часть пакетов не установлена, не являются ошибкой.

## 4. Установка Docker Engine из официального репозитория

Добавьте официальный ключ:

```bash
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
  -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc
```

Добавьте современный APT source в формате DEB822:

```bash
sudo tee /etc/apt/sources.list.d/docker.sources >/dev/null <<EOF
Types: deb
URIs: https://download.docker.com/linux/ubuntu
Suites: $(. /etc/os-release && echo "${UBUNTU_CODENAME:-$VERSION_CODENAME}")
Components: stable
Architectures: $(dpkg --print-architecture)
Signed-By: /etc/apt/keyrings/docker.asc
EOF
```

Установите Docker Engine, Buildx и Compose v2:

```bash
sudo apt update
sudo apt install -y \
  docker-ce \
  docker-ce-cli \
  containerd.io \
  docker-buildx-plugin \
  docker-compose-plugin
```

Включите автоматический запуск Docker:

```bash
sudo systemctl enable --now docker
sudo systemctl is-active docker
sudo systemctl is-enabled docker
```

Ожидаемый результат последних команд:

```text
active
enabled
```

Проверьте версии и тестовый контейнер:

```bash
sudo docker version
sudo docker compose version
sudo docker run --rm hello-world
```

Используется команда `docker compose` с пробелом. Устаревшая команда
`docker-compose` для этого проекта не нужна.

## 5. Разрешение запуска Docker без sudo

> Важно: членство в группе `docker` фактически даёт пользователю
> административный доступ к серверу. Добавляйте только доверенного
> администратора.

```bash
sudo usermod -aG docker "$USER"
```

Завершите SSH-сеанс:

```bash
exit
```

Подключитесь снова и проверьте доступ:

```bash
ssh USER@SERVER_IP
docker run --rm hello-world
docker compose version
```

`newgrp docker` можно использовать временно, но повторный вход по SSH
надёжнее и понятнее.

## 6. Сетевой доступ и firewall

Приложению нужны следующие TCP-порты:

| Порт | Назначение | Кто должен иметь доступ |
|---:|---|---|
| `22` | SSH | только администраторы |
| `8080` | Viewer Backend API | больничные агенты и доверенные клиенты |
| `3000` | OHIF Viewer | пользователи больничной сети |
| `5173` | Viewer Frontend | пользователи больничной сети |
| `8042` | Orthanc HTTP/REST | администраторы; backend использует внутреннюю Docker-сеть |
| `4242` | Orthanc DICOM | только доверенные DICOM-устройства, если нужен DIMSE-импорт |

PostgreSQL наружу не публикуется.

Docker самостоятельно создаёт правила перенаправления портов. Обычные правила
UFW могут не ограничивать опубликованные Docker-порты так, как ожидается.
Поэтому:

1. сервер должен находиться во внутренней сети или быть доступен через VPN;
2. в `.env` ниже укажите конкретный внутренний `BIND_ADDRESS`;
3. не публикуйте порты `3000`, `8042`, `4242` и `8080` напрямую в интернет;
4. для интернет-доступа сначала настройте отдельный TLS reverse proxy и
   правила `DOCKER-USER`.

Не отключайте управление iptables в Docker через `"iptables": false`: это
ломает нормальную работу bridge-сетей Compose.

Если используется облачный сервер, ограничьте эти же порты в Security Group
или сетевом firewall провайдера.

## 7. Клонирование проекта

Для серверного стека нужен только репозиторий `viewer_backend`. Репозиторий
Hospital Agent на сервер копировать не требуется.

```bash
sudo mkdir -p /opt/viewer
sudo chown "$USER:$USER" /opt/viewer
cd /opt/viewer
git clone https://github.com/repomz/viewer_backend.git
cd viewer_backend
```

Каталог проекта и файлы `.env` должны принадлежать отдельному пользователю
развёртывания, а не `root`. После настройки Docker запускайте `docker compose`
от этого пользователя без `sudo`. UID `10001` используется только внутри
контейнера backend; назначать его владельцем исходного кода на сервере не нужно.

Проверьте содержимое:

```bash
git status
docker compose config --quiet
```

Для production рекомендуется запускать заранее проверенный tag или commit, а
не произвольное состояние ветки:

```bash
git log -1 --oneline
```

## 8. Создание конфигурации `.env`

Создайте локальный файл настроек:

```bash
cp .env.compose.example .env
chmod 600 .env
```

Сгенерируйте URL-safe пароль PostgreSQL:

```bash
openssl rand -hex 24
```

Откройте файл:

```bash
nano .env
```

Проверьте и измените как минимум:

```bash
COMPOSE_PROJECT_NAME=viewer
TZ=Asia/Tomsk

# Статический внутренний IP этого сервера.
BIND_ADDRESS=10.10.10.20

POSTGRES_DB=viewer
POSTGRES_USER=viewer
POSTGRES_PASSWORD=ВСТАВЬТЕ_СГЕНЕРИРОВАННЫЙ_HEX_ПАРОЛЬ

BACKEND_PORT=8080
ORTHANC_HTTP_PORT=8042
ORTHANC_DICOM_PORT=4242
OHIF_PORT=3000
FRONTEND_PORT=5173
FRONTEND_HTTP_PORT=80
REPORTS_DIR=/app/reports

REMOTE_PACS_URL=http://pacs:8042/instances
REMOTE_PACS_USERNAME=mapdr
REMOTE_PACS_PASSWORD=ЗАМЕНИТЕ_ПАРОЛЬ_ORTHANC
REMOTE_PACS_TIMEOUT_SECONDS=300

BACKEND_IMAGE=viewer-backend:local
BACKEND_MIGRATIONS_IMAGE=viewer-backend-migrations:local
FRONTEND_IMAGE=idrisovmarat/viewer_frontend:0.1.0
IMAGE_VERSION=production
VCS_REF=COMMIT
```

Именованный volume отчётов подготавливается сервисом `reports-init`: он
назначает каталогу `/app/reports` владельца UID/GID `10001` и завершается.
Backend запускается только после успешной подготовки каталога.

Значение `BIND_ADDRESS` должно существовать на сервере:

```bash
ip -brief address
```

Для временного теста на изолированном сервере допустимо `0.0.0.0`, но это
публикует сервисы на всех интерфейсах.

Узнайте текущий commit и впишите его вместо `COMMIT`:

```bash
git rev-parse --short HEAD
```

Пароль PostgreSQL используется внутри URL `DB_DSN`, поэтому применяйте
латинские буквы и цифры без `@`, `:`, `/`, `?`, `#` и пробелов. Вывод
`openssl rand -hex 24` этому требованию соответствует.

Файл `.env` содержит секрет и не должен попадать в Git:

```bash
git check-ignore .env
```

Команда должна вывести:

```text
.env
```

## 9. Обязательная смена пароля Orthanc

В исходной конфигурации установлен тестовый пароль:

```text
mapdr / changestrongpassword
```

Перед подключением реальной больничной сети его необходимо заменить. Пароль
используется в четырёх местах:

1. `ohif-orthanc/orthanc.json` — `RegisteredUsers`;
2. `ohif-orthanc/nginx_ohif.conf` — заголовок `Authorization`;
3. `compose.yaml` — healthcheck сервиса `pacs`.
4. `.env` — `REMOTE_PACS_PASSWORD` для импорта backend → Orthanc.

Сгенерируйте отдельный пароль:

```bash
openssl rand -hex 24
```

Оставьте имя пользователя `mapdr`, замените пароль в `orthanc.json`,
healthcheck в `compose.yaml` и `REMOTE_PACS_PASSWORD` в `.env`.

Для Nginx вычислите Basic Auth:

```bash
printf '%s' 'mapdr:НОВЫЙ_ПАРОЛЬ' | base64 -w 0
```

Получившуюся строку вставьте после `Basic` в:

```nginx
proxy_set_header Authorization "Basic ПОЛУЧЕННАЯ_BASE64_СТРОКА";
```

Проверьте JSON и Compose:

```bash
jq empty ohif-orthanc/orthanc.json
docker compose config --quiet
```

Не добавляйте реальные пароли обратно в публичный Git-репозиторий.

## 10. Предварительная проверка Compose

Посмотрите список сервисов полного стека, включая опубликованный frontend:

```bash
docker compose config --services
```

Ожидается:

```text
postgres
migrations
backend
pacs
ohif
frontend
```

Проверьте итоговые образы и опубликованные адреса:

```bash
docker compose config --images
docker compose config | less
```

В итоговой конфигурации пароль PostgreSQL отображается открытым текстом.
Не отправляйте вывод `docker compose config` посторонним.

## 11. Первый запуск

Из каталога `/opt/viewer/viewer_backend` выполните одну команду:

```bash
docker compose pull frontend
docker compose up -d --build --wait --remove-orphans
```

Первая сборка и загрузка образов могут занять несколько минут.

Проверьте состояние:

```bash
docker compose ps -a
```

Ожидаемое состояние:

- `postgres` — `Up (healthy)`;
- `backend` — `Up`;
- `pacs` — `Up (healthy)`;
- `ohif` — `Up` или `Up (healthy)`;
- `frontend` — `Up`;
- `migrations` — `Exited (0)`.

`migrations: Exited (0)` является успешным результатом, а не ошибкой.

Проверьте логи миграций:

```bash
docker compose logs migrations
```

В конце должна быть строка об успешном применении миграций.

## 12. Проверка сервисов на самом сервере

Подставьте адрес из `BIND_ADDRESS`:

```bash
SERVER_IP=10.10.10.20
```

Backend:

```bash
curl --fail --show-error "http://${SERVER_IP}:8080/"
```

Ожидается:

```text
DICOM viewer API v0.1
```

Frontend:

```bash
curl --fail --show-error "http://${SERVER_IP}:5173/healthz"
```

Ожидается:

```text
ok
```

Orthanc:

```bash
curl --fail --show-error \
  --user 'mapdr:НОВЫЙ_ПАРОЛЬ' \
  "http://${SERVER_IP}:8042/system" | jq '{Name, Version, DicomAet, DicomPort}'
```

OHIF:

```bash
curl --fail --show-error --head "http://${SERVER_IP}:3000/"
```

Проверка DICOM-порта:

```bash
nc -vz "${SERVER_IP}" 4242
```

Если `nc` отсутствует:

```bash
sudo apt install -y netcat-openbsd
```

Проверка открытого сокета не заменяет DICOM C-ECHO. Полноценный C-ECHO
выполняется с больничного агента или DICOM-утилитой.

## 13. Проверка с другого компьютера

С доверенного компьютера больничной сети проверьте:

- `http://SERVER_IP:3000` — OHIF Viewer;
- `http://SERVER_IP:8042` — Orthanc;
- `http://SERVER_IP:8080` — Viewer Backend.

Параметры DICOM:

```text
Host:     SERVER_IP
Port:     4242
AE Title: MAPDR
```

Если локальные проверки проходят, а удалённые нет, проверьте:

- значение `BIND_ADDRESS`;
- маршрутизацию между подсетями;
- firewall сервера;
- Security Group облачного провайдера;
- отсутствие NAT-конфликта портов.

## 14. Настройка больничного Hospital Agent

Hospital Agent устанавливается отдельно на Windows-компьютере.

В его `agent_config.json`:

```json
{
  "viewer_url": "http://SERVER_IP:8080"
}
```

В `config.json` указывается локальный больничный PACS, из которого агент
выполняет C-FIND и C-GET:

```json
{
  "pacs": {
    "ip": "HOSPITAL_PACS_IP",
    "port": 11112,
    "ae_title": "HOSPITAL_PACS_AE"
  },
  "local": {
    "ae_title": "HOSPITAL_AGENT"
  }
}
```

Yandex Object Storage задаётся переменными окружения на больничном компьютере:

```text
YANDEX_ACCESS_KEY_ID
YANDEX_SECRET_ACCESS_KEY
YANDEX_BUCKET
YANDEX_ENDPOINT
```

Адрес серверного Orthanc в команды агента не передаётся: backend берёт его из
`REMOTE_PACS_URL`.

Запуск без консольного окна:

```powershell
C:\path\to\.venv\Scripts\pythonw.exe C:\path\to\agent\hospital_agent.py
```

На сервере должен появиться heartbeat агента, а команды `/user_requests`
будут выполняться на больничном компьютере.

## 15. Повседневное управление

Состояние:

```bash
cd /opt/viewer/viewer_backend
docker compose ps -a
```

Последние логи:

```bash
docker compose logs --tail=200 backend
docker compose logs --tail=200 pacs
docker compose logs --tail=200 ohif
docker compose logs --tail=200 postgres
```

Логи в реальном времени:

```bash
docker compose logs -f backend pacs ohif
```

Перезапуск одного сервиса:

```bash
docker compose restart backend
```

Остановка с сохранением данных:

```bash
docker compose down
```

Повторный запуск:

```bash
docker compose up -d --wait --remove-orphans
```

Никогда не используйте следующую команду без подтверждённой резервной копии:

```bash
docker compose down --volumes
```

Она удаляет базу PostgreSQL, DICOM-хранилище Orthanc и сохранённые JSON-отчёты.

## 16. Резервное копирование

Создайте каталог, недоступный обычным пользователям:

```bash
sudo mkdir -p /var/backups/viewer
sudo chmod 700 /var/backups/viewer
```

### PostgreSQL

```bash
cd /opt/viewer/viewer_backend
BACKUP_FILE="/var/backups/viewer/postgres-$(date +%F-%H%M%S).dump"
set -o pipefail
docker compose exec -T postgres \
  sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' \
  | sudo tee "$BACKUP_FILE" >/dev/null
```

Проверьте, что файл создан и не пуст:

```bash
sudo ls -lh /var/backups/viewer
```

### Orthanc

Для согласованной файловой копии кратковременно остановите PACS:

```bash
cd /opt/viewer/viewer_backend
docker compose stop pacs
sudo docker run --rm \
  -v viewer_orthanc-data:/source:ro \
  -v /var/backups/viewer:/backup \
  alpine:3.22 \
  sh -c 'tar -czf /backup/orthanc-$(date +%F-%H%M%S).tar.gz -C /source .'
docker compose up -d --wait pacs
```

### JSON-отчёты

```bash
cd /opt/viewer/viewer_backend
sudo docker run --rm \
  -v viewer_reports-data:/source:ro \
  -v /var/backups/viewer:/backup \
  alpine:3.22 \
  sh -c 'tar -czf /backup/reports-$(date +%F-%H%M%S).tar.gz -C /source .'
```

Скопируйте резервные копии на другой сервер или защищённое хранилище.
Копия на том же физическом диске не защищает от отказа диска.

Периодически проверяйте восстановление на отдельном тестовом сервере.

### Проверка PostgreSQL-копии

Проверить структуру dump-файла без восстановления:

```bash
cat /var/backups/viewer/postgres-ДАТА.dump \
  | docker compose exec -T postgres pg_restore --list \
  | head
```

Полное восстановление выполняйте только на отдельном тестовом сервере или
после подтверждённой остановки приложения. `pg_restore --clean` удаляет
существующие объекты целевой базы:

```bash
docker compose stop backend
cat /var/backups/viewer/postgres-ДАТА.dump \
  | docker compose exec -T postgres \
      sh -c 'pg_restore --clean --if-exists -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
docker compose up -d --wait backend
```

Перед восстановлением Orthanc остановите `ohif` и `pacs`, очистите целевой
volume и распакуйте архив. Делайте это только на новом или подтверждённо
восстанавливаемом сервере:

```bash
docker compose stop ohif pacs
sudo docker run --rm \
  -v viewer_orthanc-data:/target \
  -v /var/backups/viewer:/backup:ro \
  alpine:3.22 \
  sh -c 'rm -rf /target/* /target/.[!.]* /target/..?*; tar -xzf /backup/orthanc-ДАТА.tar.gz -C /target'
docker compose up -d --wait pacs ohif
```

## 17. Обновление приложения

Перед обновлением создайте резервную копию.

```bash
cd /opt/viewer/viewer_backend
git status
git pull --ff-only
git log -1 --oneline
docker compose pull --ignore-buildable
docker compose up -d --build --wait --remove-orphans
docker compose ps -a
```

Если на сервере вручную изменены файлы Orthanc или Compose для секретов,
`git pull --ff-only` может остановиться из-за локальных изменений. Не
перезаписывайте их вслепую: сначала сохраните конфигурацию и перенесите
изменения в новую версию.

Не выполняйте автоматическое обновление major-версий Docker и образов без
проверки на тестовом сервере.

## 18. Диагностика

Общая информация:

```bash
docker version
docker compose version
docker info
docker compose ps -a
docker system df
df -h
```

Если сервис не стартовал:

```bash
docker compose logs --tail=300 SERVICE
docker compose ps -q SERVICE | xargs --no-run-if-empty docker inspect
```

Если миграция завершилась с ненулевым кодом:

```bash
docker compose logs migrations
docker compose up migrations
```

Если порт уже занят:

```bash
sudo ss -lntp | grep -E ':(3000|4242|8042|8080)\b'
```

Если заканчивается место:

```bash
df -h
docker system df
sudo du -sh /var/lib/docker
```

Не запускайте `docker system prune --volumes`: команда может удалить данные,
которые не используются запущенным контейнером в данный момент.

## 19. Контрольный список перед эксплуатацией

- [ ] Сервер использует статический IP или постоянное DNS-имя.
- [ ] Установлен Docker Engine из официального репозитория.
- [ ] Работает `docker compose` v2.
- [ ] Docker включён в автозагрузку.
- [ ] В `.env` задан URL-safe пароль PostgreSQL.
- [ ] `BIND_ADDRESS` указывает на внутренний интерфейс.
- [ ] Тестовый пароль Orthanc заменён во всех трёх местах.
- [ ] Порты не опубликованы напрямую в интернет.
- [ ] Все постоянные сервисы имеют статус `Up`.
- [ ] Контейнер миграций завершился с `Exited (0)`.
- [ ] Backend, OHIF, Orthanc и DICOM-порт доступны из больничной сети.
- [ ] Hospital Agent отправляет heartbeat.
- [ ] Настроено регулярное резервное копирование.
- [ ] Резервные копии хранятся вне основного сервера.
- [ ] Проверено восстановление на тестовой машине.



## 20. Guide to renew containers // не исправлять!

```bash

cd opt/viewer/viewer_backend

git status
git stash  # если есть untracked files

docker compose ps

docker compose down

docker stop $(docker ps -aq).  # если какие то контейнеры не удаляются - Resource is still in use
docker rm -f $(docker ps -aq)  # если какие то контейнеры не удаляются - Resource is still in use

# Не добавляйте --volumes при обновлении: этот ключ удалит БД и PACS.

git pull

git stash drop

git status

cp .env.compose.example .env

cat .env

chmod 600 .env

grep FRONTEND_IMAGE .env

docker compose pull frontend

docker compose up -d --build --wait --remove-orphans

```
