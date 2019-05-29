ARG ARCH="amd64"
ARG OS="linux"
FROM quay.io/dnxware/busybox-${OS}-${ARCH}:latest
LABEL maintainer="The dnxware Authors <dnxware-developers@googlegroups.com>"

ARG ARCH="amd64"
ARG OS="linux"
COPY .build/${OS}-${ARCH}/dnxware        /bin/dnxware
COPY .build/${OS}-${ARCH}/promtool          /bin/promtool
COPY documentation/examples/dnxware.yml  /etc/dnxware/dnxware.yml
COPY console_libraries/                     /usr/share/dnxware/console_libraries/
COPY consoles/                              /usr/share/dnxware/consoles/

RUN ln -s /usr/share/dnxware/console_libraries /usr/share/dnxware/consoles/ /etc/dnxware/
RUN mkdir -p /dnxware && \
    chown -R nobody:nogroup etc/dnxware /dnxware

USER       nobody
EXPOSE     9090
VOLUME     [ "/dnxware" ]
WORKDIR    /dnxware
ENTRYPOINT [ "/bin/dnxware" ]
CMD        [ "--config.file=/etc/dnxware/dnxware.yml", \
             "--storage.tsdb.path=/dnxware", \
             "--web.console.libraries=/usr/share/dnxware/console_libraries", \
             "--web.console.templates=/usr/share/dnxware/consoles" ]
