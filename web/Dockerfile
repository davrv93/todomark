# Build stage
FROM node:20-alpine AS builder
WORKDIR /app
ARG VITE_API_URL
ENV VITE_API_URL=$VITE_API_URL
ARG VITE_HYDRA_CLIENT_ID
ENV VITE_HYDRA_CLIENT_ID=$VITE_HYDRA_CLIENT_ID
ARG VITE_HYDRA_AUTH_URL
ENV VITE_HYDRA_AUTH_URL=$VITE_HYDRA_AUTH_URL
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