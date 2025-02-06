psql -U postgres -f init.sql
go build
./rss-app migrate
