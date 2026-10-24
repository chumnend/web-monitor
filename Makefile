.PHONY: start stop up down clean clean-all

all: up

start:
	@docker compose start

stop:
	@docker compose stop

status:
	@docker compose ps

up: 
	@docker compose up -d --build

down:
	@docker compose down

# Danger zone: nukes volumes too (wipes DB data)
clean:
	@docker compose down -v

# Nukes volumes AND removes orphan containers/images tied to this project
clean-all:
	@docker compose down -v --remove-orphans --rmi local