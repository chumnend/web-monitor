# Stage 1: Build the React app

FROM node:24-alpine AS builder

WORKDIR /app/frontend

RUN corepack enable

COPY package.json yarn.lock ./

RUN yarn install

COPY . .

RUN yarn build

# Stage 2: Serve with nginx

FROM nginx:alpine

COPY --from=builder /app/frontend/dist /usr/share/nginx/html

RUN <<'EOF' cat > /etc/nginx/conf.d/default.conf
server {
    listen 80;
    server_name _;
    root /usr/share/nginx/html;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }

    location ~* \.(js|css|png|jpg|jpeg|gif|svg|ico|woff2?)$ {
        expires 30d;
        add_header Cache-Control "public, immutable";
    }
}
EOF

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
