build-java:
	@javac -d . HelloGraal.java

build-executable: build-java
	@native-image -cp . co.elastic.profiling.HelloGraal hellograal

clean:
	@rm -rf hellograal co
