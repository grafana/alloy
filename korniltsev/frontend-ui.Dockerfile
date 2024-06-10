
FROM  node:20 as build

#cd ./internal/web/ui && yarn --network-timeout=1200000 && yarn run build
WORKDIR /src/ui
COPY ./internal/web/ui/package.json ./internal/web/ui/yarn.lock /src/ui/

RUN yarn --network-timeout=1200000

COPY ./internal/web/ui /src/ui

RUN yarn run build

FROM scratch
COPY --from=build /src/ui/build /