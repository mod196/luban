FROM node:16.20-alpine AS builder

WORKDIR /src

COPY package.json ./
RUN npm install --no-package-lock --legacy-peer-deps --registry=https://registry.npmjs.org --no-audit --no-fund \
    && npm install --no-save --no-package-lock --legacy-peer-deps --registry=https://registry.npmjs.org --no-audit --no-fund \
      vue@3.1.5 @vue/compiler-sfc@3.1.5 vue-router@4.0.10 vuex@4.0.2 xterm@4.15.0 xterm-addon-search@0.8.1

COPY . .
RUN npm run buildprod \
    && find /src/dist -type f \( -name '*.js' -o -name '*.css' -o -name '*.html' -o -name '*.svg' -o -name '*.json' \) \
      -exec sh -c 'for file do gzip -9 -c "$file" > "$file.gz"; done' sh {} +

FROM nginx:1.25-alpine

COPY nginx.conf /etc/nginx/nginx.conf
COPY --from=builder /src/dist /apps/luban-fe/dist

EXPOSE 80
