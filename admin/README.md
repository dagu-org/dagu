# dagu Admin UI

## How to develop?

### 1. start backend server

The backend server will start at `127.0.0.1:8080` (default).
```
cd <project-root>
make server
```

### 2. start webpack-dev-server

The webpack dev server will start at `127.0.0.1:8081`.
```
cd admin/
yarn install
yarn start
```

Then browse to [http://localhost:8081](http://localhost:8081).

## How to build bundle.js?

The below command will build `bundle.js` and copy it to `<project>/internal/admin/handlers/web/assets/js/bundle.js` so that go can include the javascript within the binary.

```
cd <project-root>
make build-admin
```

## How to contribute?

Feel free to contribute in any way you want. Share ideas, questions, submit issues, and create pull requests. Thanks!
