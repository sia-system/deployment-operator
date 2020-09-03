FROM debian:buster-slim
WORKDIR /

ADD ./deployment-operator /

RUN adduser -u 1001 --disabled-password --no-create-home --gecos "" app_user

RUN chmod +x ./deployment-operator
RUN chown 1001 ./deployment-operator

USER app_user

CMD ["./deployment-operator"]