# Fast Pay 前端静态站 + 后端反代（nginx 容器内监听 4000）
# 构建：在仓库根执行  docker build -t epay-web .
# 运行（后端与本容器同宿主、监听 127.0.0.1:9200）：
#   docker run -d --network host --name epay-web epay-web
#   （host 网络下容器内 127.0.0.1:9200 即宿主后端；nginx 直接占用宿主 4000）
# 若后端是另一个容器：改用 bridge + 后端服务名，并把 default.conf 的
#   127.0.0.1:9200 换成 <后端服务名>:9200，run 时 -p 4000:4000。

# ---- 构建前端 ----
FROM node:22-alpine AS build
WORKDIR /app

COPY frontend/package.json frontend/package-lock.json* ./
RUN npm ci

COPY frontend/ .
RUN npm run build

# ---- 运行 ----
FROM nginx:1.27-alpine
# 选 nginx conf：默认 host 网络版（127.0.0.1:9200）；
# compose 网络里用 docker build --build-arg NGINX_CONF=compose.conf（反代 server:9200）
ARG NGINX_CONF=default.conf
COPY --from=build /app/dist /usr/share/nginx/html
COPY deploy/nginx/${NGINX_CONF} /etc/nginx/conf.d/default.conf
EXPOSE 4000
