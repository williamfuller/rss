./init-db.sh

curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-arm64
chmod +x tailwindcss-linux-arm64
mv tailwindcss-linux-arm64 tailwindcss

wget -O static/htmx.js https://unpkg.com/htmx.org@2.0.2/dist/htmx.min.js

go build .
./rss migrate
