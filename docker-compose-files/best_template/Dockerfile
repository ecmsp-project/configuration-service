FROM maven:3.9-eclipse-temurin-21 AS builder
WORKDIR /app
COPY pom.xml .
COPY src ./src
RUN mvn clean package -DskipTests -DfailOnNoContracts=false

FROM eclipse-temurin:21-jdk AS runtime
WORKDIR /app

COPY --from=builder /app/target/*.jar ./app.jar
CMD ["java", "-jar", "app.jar"]