FROM golang:1.23 AS build
WORKDIR /src
COPY services/usersvc/ ./services/usersvc/
WORKDIR /src/services/usersvc
RUN go mod tidy && CGO_ENABLED=0 go build -o /out/usersvc

FROM gcr.io/distroless/base-debian12
COPY --from=build /out/usersvc /usersvc
EXPOSE 8080
ENTRYPOINT ["/usersvc"]
