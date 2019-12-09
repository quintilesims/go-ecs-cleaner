FROM alpine:3.1

RUN apk update && apk add ca-certificates
ADD ./build/Linux/go-ecs-cleaner /usr/local/bin/go-ecs-cleaner
RUN chmod +x /usr/local/bin/go-ecs-cleaner

ENV FLAGS ""
ENV AWS_ACCESS_KEY ""
ENV AWS_SECRET_ACCESS_KEY ""
ENV AWS_REGION ""

ENTRYPOINT ["sh", "-c", "go-ecs-cleaner ecs-task ${FLAGS}"]