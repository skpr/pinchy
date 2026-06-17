FROM ghcr.io/berriai/litellm:main-stable

COPY images/litellm/config.yml /app/config.yml

EXPOSE 4000

CMD ["--config", "/app/config.yml", "--port", "4000"]
