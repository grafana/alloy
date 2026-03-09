FROM eclipse-temurin:17-jdk-jammy

ADD ./FastSlow.java /FastSlow.java
RUN javac FastSlow.java

CMD ["java", "FastSlow"]
