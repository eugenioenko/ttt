FROM debian:trixie-slim
RUN apt-get update && apt-get install -y --no-install-recommends gcc libc6-dev gdb unzip libpython3.13 libxml2 && rm -rf /var/lib/apt/lists/*
COPY codelldb-linux-x64.vsix /tmp/codelldb.zip
RUN echo '1cd7f386598022b51a5b93b9ffa23e812b23f519cfe1833384ec4bef4bfd1be1  /tmp/codelldb.zip' | sha256sum -c - && unzip -q /tmp/codelldb.zip 'extension/adapter/*' 'extension/lldb/*' -d /opt && rm /tmp/codelldb.zip
ENV PATH="/opt/extension/adapter:${PATH}"
