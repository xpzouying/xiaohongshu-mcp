FROM node:22-alpine

WORKDIR /app

COPY apps/content-remix/web/package.json /app/package.json
RUN npm install

COPY apps/content-remix/web /app

EXPOSE 5173

CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0", "--port", "5173"]

