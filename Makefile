NAME = Transcendence

all: build up

re: down clean build up

help:
	@echo "=== Transcendence Project ==="
	@echo ""
	@echo "Available commands:"
	@echo "  make build              - Build les images Docker"
	@echo "  make up                 - Lance les containers"
	@echo "  make down               - Arrête les containers"
	@echo "  make logs               - Affiche les logs"
	@echo "  make stop               - Stop les containers"
	@echo "  make restart            - Redémarre les containers"
	@echo "  make clean              - Remove les containers et volumes"
	@echo "  make migrate            - Lance les migrations Django"
	@echo "  make makemigrations     - Crée les fichiers de migration"
	@echo "  make create-superuser   - Crée un superuser (admin)"
	@echo "  make shell-backend      - Accède au shell Django"
	@echo "  make shell-db           - Accède au shell PostgreSQL"
	@echo ""

build:
	docker compose build

up:
	docker compose up -d
	@echo "✅ Services démarrés!"
	@echo "Django: http://localhost:8000"
	@echo "React: http://localhost:3000"
	@echo "Admin Django: http://localhost:8000/admin"

down:
	docker compose down -v
	@echo "✅ Services arrêtés"

logs:
	docker compose logs -f

stop:
	docker compose stop
	@echo "✅ Services stoppés"

restart: down up

clean:
	docker compose down -v
	@echo "✅ Containers et volumes supprimés"
