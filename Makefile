start: 
	docker compose up -d --build

stop:
	docker compose stop

restart: stop start

down:
	docker compose down

logs:
	docker compose logs -f

# Danger zone: nukes volumes too (wipes DB data)
clean:
	docker compose down -v

# Nukes volumes AND removes orphan containers/images tied to this project
clean-all:
	docker compose down -v --remove-orphans --rmi local