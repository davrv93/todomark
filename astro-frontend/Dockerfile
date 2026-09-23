# Build stage
FROM node:22-alpine AS builder
WORKDIR /app
ARG PUBLIC_API_URL
ARG PUBLIC_HYDRA_CLIENT_ID
ARG PUBLIC_HYDRA_AUTH_URL
ARG PUBLIC_DIRECTUS_URL
ENV PUBLIC_API_URL=$PUBLIC_API_URL
ENV PUBLIC_HYDRA_CLIENT_ID=$PUBLIC_HYDRA_CLIENT_ID
ENV PUBLIC_HYDRA_AUTH_URL=$PUBLIC_HYDRA_AUTH_URL
ENV PUBLIC_DIRECTUS_URL=$PUBLIC_DIRECTUS_URL
COPY package.json ./
RUN npm install
COPY . .
RUN npm run build

# Production stage (nginx)
FROM nginx:alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist /usr/share/nginx/html
COPY docker-entrypoint.d/40-runtime-config.sh /docker-entrypoint.d/40-runtime-config.sh
RUN chmod +x /docker-entrypoint.d/40-runtime-config.sh
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
