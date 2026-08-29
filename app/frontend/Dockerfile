# Stage 1: Build the React app

FROM node:24-alpine AS builder

WORKDIR /app/frontend

RUN apk add --no-cache yarn

COPY package.json yarn.lock ./

RUN yarn install

COPY . .

RUN yarn build

# Stage 2: Serve files with nginx

FROM nginx:alpine

COPY --from=builder /app/frontend/dist /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
