I am using golang-migrate to manage db migrations for this project. 

New migrations are created using, 

```
migrate create -ext sql -dir ./migrations -seq <task_name>
```

Run migrations using,

```
migrate -path ./migrations/ -database "postgresql://<user>:<password>@localhost:5432/<database>" up
```

Revert migrations usings,

```
migrate -path ./migrations/ -database "postgresql://<user>:<password>@localhost:5432/<database>" down
```
