FROM mcr.microsoft.com/dotnet/aspnet:8.0 AS base
USER $APP_UID
WORKDIR /app
EXPOSE 8080
EXPOSE 8081

FROM mcr.microsoft.com/dotnet/sdk:8.0 AS build
ARG BUILD_CONFIGURATION=Release
WORKDIR /src
COPY ["Deckard.Benchmark.Api/Deckard.Benchmark.Api.csproj", "Deckard.Benchmark.Api/"]
RUN dotnet restore "Deckard.Benchmark.Api/Deckard.Benchmark.Api.csproj"
COPY . .
WORKDIR "/src/Deckard.Benchmark.Api"
RUN dotnet build "Deckard.Benchmark.Api.csproj" -c $BUILD_CONFIGURATION -o /app/build

FROM build AS publish
ARG BUILD_CONFIGURATION=Release
RUN dotnet publish "Deckard.Benchmark.Api.csproj" -c $BUILD_CONFIGURATION -o /app/publish /p:UseAppHost=false

FROM base AS final
WORKDIR /app
COPY --from=publish /app/publish .
ENTRYPOINT ["dotnet", "Deckard.Benchmark.Api.dll"]
