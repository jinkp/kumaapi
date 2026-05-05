# kumaapi

`kumaapi` es un cliente y CLI en Go para interactuar con [Uptime Kuma](https://github.com/louislam/uptime-kuma). El binario `kuma` permite autenticarse, consultar estado y administrar recursos comunes como monitores, tags y API keys desde terminal.

## Compatibilidad

- Uptime Kuma v2.x
- Probado en Uptime Kuma 2.3.2
- Go 1.26

## Instalación

### Instalar el CLI

```bash
go install github.com/jinkp/kumaapi/cmd/kuma@latest
```

### Compilar localmente

```bash
go build ./...
```

## Configuración

Primero autentícate contra tu instancia de Uptime Kuma:

```bash
kuma login --url http://localhost:3002 --user admin --password secret
```

El comando guarda la configuración en `~/.kuma/config.yaml`.

También puedes sobrescribir valores por comando:

```bash
kuma --url http://localhost:3002 --token <jwt> status
```

## Comandos disponibles

### Ver ayuda general

```bash
kuma --help
```

### Ver versión

```bash
kuma version
```

### Estado de la instancia

```bash
kuma status
kuma status --output json
```

### Monitores

```bash
kuma monitor list
kuma monitor get 1
kuma monitor add --type http --name homepage --url https://example.com --interval 60 --method GET
kuma monitor pause 1
kuma monitor resume 1
kuma monitor delete 1
kuma monitor watch 1
```

### Tags

```bash
kuma tag list
kuma tag add --name production --color '#00AAFF'
kuma tag delete 1
```

### API keys

```bash
kuma apikey list
kuma apikey add --name ci-bot
kuma apikey enable 1
kuma apikey disable 1
kuma apikey delete 1
```

## Desarrollo

### Levantar Uptime Kuma con Docker

El proyecto incluye `docker/docker-compose.yml` para entorno local:

```bash
docker compose -f docker/docker-compose.yml up -d
```

La instancia queda disponible en `http://localhost:3002`.

### Ejecutar validaciones locales

#### Build

```bash
go build ./...
```

#### Vet

```bash
go vet ./...
```

#### Tests rápidos

Los tests de integración se saltan en modo corto:

```bash
go test ./... -short -count=1
```

#### Tests de integración completos

Requieren Docker y una instancia funcional de Uptime Kuma:

```bash
go test ./tests/integration/... -count=1 -v
```

Variables de entorno soportadas por integración:

- `KUMA_URL` (default: `http://localhost:3002`)
- `KUMA_USER`
- `KUMA_PASS`

## CI y releases

- `CI`: compila, ejecuta `go vet` y corre `go test ./... -short -count=1`
- `Release`: publica binarios multiplataforma con GoReleaser al hacer push de tags `v*`

## Licencia

MIT
