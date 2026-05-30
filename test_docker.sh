docker build -t goboxd-test --target go-builder .
# wait, go-builder has /usr/local/go/bin/go.
# The final runtime image:
docker build -t goboxd-runtime .
docker run --rm --cgroupns=host goboxd-runtime /bin/bash -c "which go"
