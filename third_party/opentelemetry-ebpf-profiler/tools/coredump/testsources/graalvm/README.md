## Testing GraalVM native images

### Pre-requisites

1. [download and install GraalVM for Linux](https://www.graalvm.org/22.3/docs/getting-started/linux/) 
2. setup GraalVM as default Java runtime
3. install the `native-image` compiler, see more details in the 
  [docs](https://www.graalvm.org/22.3/reference-manual/native-image/#install-native-image)

### Run the experiment

- Build a native image executable from the code in HelloGraal.java
    ```bash
    make build-executable
    ```
- Start the host-agent
- Run the `hellograal` binary
