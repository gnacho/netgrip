# NetGrip

<p align="center">
  <a href="README.md">English</a> |
  <a href="README.es.md">Español</a>
</p>

<p align="center">
  <a href="https://netgrip.cloudless.club"><img alt="Sitio web" src="https://img.shields.io/badge/website-netgrip.cloudless.club-2E6BE6"></a>
  <a href="https://demo.netgrip.cloudless.club"><img alt="Demo en vivo" src="https://img.shields.io/badge/demo-demo.netgrip.cloudless.club-0D9488"></a>
  <a href="https://github.com/gnacho/netgrip/releases"><img alt="Release" src="https://img.shields.io/github/v/release/gnacho/netgrip"></a>
  <a href="LICENSE"><img alt="Licencia" src="https://img.shields.io/github/license/gnacho/netgrip"></a>
</p>

<p align="center">
  <strong>Cada servicio de tu router OpenWrt, detrás de un interruptor que
  puede pulsar tu familia.</strong><br>
  NetGrip es un panel companion que corre en el propio router, junto a LuCI:
  WireGuard, WiFi de invitados, QoS, DNS y más, un clic cada uno, con
  snapshot, comprobación de salud y rollback automático si algo sale mal.
</p>

<p align="center">
  <a href="https://demo.netgrip.cloudless.club"><strong>Prueba la demo en vivo</strong></a> ·
  <a href="https://netgrip.cloudless.club"><strong>Visita la web</strong></a>
</p>

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/hero-es-dark.png">
    <img alt="Resumen de NetGrip: tarjetas de sistema y WAN, gráfico de tráfico en vivo y panel de puertos ethernet, en tema claro" src="assets/hero-es-light.png" width="800">
  </picture>
</p>

## Míralo antes de instalarlo

- **[demo.netgrip.cloudless.club](https://demo.netgrip.cloudless.club)** corre
  el panel real con datos de ejemplo: pulsa cada interruptor, abre cada
  tarjeta, no se aplica nada en ningún sitio.
- **[netgrip.cloudless.club](https://netgrip.cloudless.club)** lista
  **todas y cada una de las funcionalidades**, pantalla a pantalla, con el
  razonamiento detrás de cada una.

## ¿Por qué NetGrip?

Seguía entregándoles routers OpenWrt a personas que solo quieren WiFi y una
VPN que funcionen. LuCI es una herramienta para ingenieros: cada toggle te
pide una sección, una interfaz y una opción. Los portales del fabricante son
más amables, pero son apps cerradas que solo funcionan en su firmware. Yo
quería la máquina intermedia: el router de verdad, detrás de botones que
pueda pulsar un familiar. NetGrip toma los servicios que LuCI ya expone vía
rpcd y les pone un interruptor delante. Lo pulsas, y NetGrip hace snapshot
de la config, aplica el cambio, espera, comprueba que el servicio levantó, y
se deshace solo si no fue así.

Tres reglas lo dan forma:

- **Vive en el router.** Un único binario Go estático con la UI embebida
  (unos 10 MB en disco y ~15 MB de RAM en uso, medidos en un router ARM64;
  ~11,4 MB en mipsle), empaquetado como `.apk`/`.ipk` real de OpenWrt que
  sobrevive al sysupgrade. Sin contenedores, sin caja extra, sin Node en un
  router.
- **Seguro por construcción.** Cada cambio pasa por un executor con
  allowlist, con snapshot de config y rollback automático. El panel no puede
  salirse del camino trazado.
- **Sin cuentas nuevas.** El login se valida contra rpcd, la misma sesión
  que usa LuCI; un admin por router, nada extra que administrar. AGPL-3.0,
  sin premium de ningún tipo.

## Qué te llevas

**Un resumen que responde a "¿está todo bien?"** Salud del sistema, estado
WAN y tráfico en vivo en una sola pantalla, más un chasis ethernet con
estado por puerto, nombres de dispositivo y detección de switches no
gestionados. Es la captura de arriba.

**Clientes que se pueden gestionar de verdad.** Cada estación con velocidad
y señal, reservas de IP en un clic, bloqueo y límites de ancho de banda por
dispositivo.

<p align="center">
  <img alt="Página de clientes: tabla ordenable con nombres, tipo de conexión, señal y uso por cliente" src="assets/screenshot-clients-es.png" width="800">
</p>

**Una tarjeta por servicio, cada una con su interruptor real.** WireGuard
(peers con código QR) y OpenVPN (`.ovpn` listo para descargar), DNS con
protección rebind, SQM basado en cake con nota de bufferbloat, DDNS, WiFi
de invitados e IoT en subredes aisladas, reenvío de puertos y un conmutador
router/AP que mueve el rol WAN sin editar `network` y `firewall` a mano.

<p align="center">
  <img alt="Página de servicios con tarjetas de WireGuard, DDNS, SQM y el firewall visual" src="assets/screenshot-services-es.png" width="800">
</p>

**Cuidado del sistema sin ceremonia.** Acceso y sesión, tarjetas de
seguridad, asistente de primer arranque, actualizaciones de firmware vía
owut/ASU que reconstruyen la imagen con tus paquetes dentro, y la malla de
roaming dibujada como grafo radial vía usteer.

<p align="center">
  <img alt="Página de sistema con acceso, seguridad, modo Router/AP y tarjetas de actualización" src="assets/screenshot-system-es.png" width="800">
</p>

**Y más:** análisis de tráfico avanzado por aplicación (netifyd) con
timeline de 24 h, la entrada opcional `luci-app-netgrip` bajo LuCI >
Servicios, e interfaz ES/EN que cambia en un clic. Y esto todavía no es
todo: **[en la web están todas las
funcionalidades](https://netgrip.cloudless.club)**, cada una con sus
capturas a tamaño real.

**Agente de NetPulse integrado.** El mismo binario reporta métricas, eventos
WiFi y clientes a [NetPulse](https://netpulse.cloudless.club): el router
aparece etiquetado como NetGrip en su flota sin instalar nada más. Es una
capacidad adicional, no un requisito: si no usas NetPulse, nada cambia.

## Ponlo en tu router

Requisitos: un router ARM de 64 bits (`aarch64_cortex-a53`, cubre MediaTek
filogic y Qualcomm ipq807x) o x86_64, con OpenWrt 24.10 o 25.12. El panel
escucha en el puerto 8090 y entra con tus credenciales de LuCI.

SSH al router y ejecuta:

```sh
wget -qO- https://raw.githubusercontent.com/gnacho/netgrip/main/install.sh | sh
```

El script elige el paquete correcto para la arquitectura y el gestor del
router (`apk` en OpenWrt 25.12+, `opkg` en 24.10), instala la última
release, habilita y arranca el servicio, e imprime la URL del panel.
¿Prefieres revisarlo antes? Lee [install.sh](install.sh). Para fijar
versión: `NETGRIP_VERSION=vX.Y.Z sh install.sh`.

Abre `http://<ip-del-router>:8090` y entra con el mismo usuario y contraseña
que usas en LuCI.

<details>
<summary><strong>Instalación manual (paquetes)</strong></summary>

Descarga el paquete correcto desde la página de
[releases](https://github.com/gnacho/netgrip/releases/latest) y, en el
router:

```sh
# OpenWrt 25.12 (apk)
apk add netgrip-<version>-r1-arm64.apk

# OpenWrt 24.10 (ipk)
opkg install netgrip_<version>-1_aarch64_cortex-a53.ipk

/etc/init.d/netgrip enable && /etc/init.d/netgrip start
```

El postinst añade el binario, el init y el enlace rc.d a
`/etc/sysupgrade.conf`, así que el panel vuelve tras una actualización de
firmware. El paquete opcional `luci-app-netgrip` incrusta el panel bajo LuCI
> Servicios de la misma forma.

</details>

<details>
<summary><strong>Instalación manual (binario suelto) y build desde fuente</strong></summary>

Un binario suelto funciona, pero muere en cada actualización de firmware;
el paquete es el camino recomendado.

```sh
# dropbear de busybox no tiene scp; mejor por pipe:
cat netgrip-linux-arm64 | ssh root@<ip-del-router> "cat > /usr/sbin/netgrip && chmod 755 /usr/sbin/netgrip"
/usr/sbin/netgrip -listen 0.0.0.0 -port 8090
```

Compilar desde fuente requiere Go 1.24+ y Node 22+:

```sh
git clone https://github.com/gnacho/netgrip.git
cd netgrip
cd app && npm ci && cd ..
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o netgrip ./cmd/netgrip
```

</details>

<details>
<summary><strong>Referencia para curiosos: flags, servicio y API JSON</strong></summary>

Flags (los defaults sirven para casi cualquier router):

| Flag | Default | Descripción |
| --- | --- | --- |
| `-listen` | `0.0.0.0` | Dirección a la que bindear. |
| `-port` | `8090` | Puerto. El firmware GL.iNet sirve su propia web en el 8080, de ahí el default. |
| `-rpcd-url` | `http://127.0.0.1/ubus` | Endpoint JSON-RPC de rpcd para validar el login. |

El timeout de sesión se cambia desde la tarjeta Acceso de la UI
(`options.main.session_timeout` en UCI). El servicio corre con procd:

```sh
/etc/init.d/netgrip status
logread -e netgrip -f
/etc/init.d/netgrip restart
```

Detrás de cada tarjeta hay una API JSON, la misma que usa el frontend:
lecturas por `GET /api/board`, `/api/system`, `/api/wan`, `/api/wifi`,
`/api/lan`, `/api/dns`, `/api/usteer`, `/api/clients`, `/api/netifyd`,
`/api/dpi/apps`, `/api/dpi/timeline` y `/api/nftqos`; escrituras por los
`POST` correspondientes (`/api/wireguard`, `/api/openvpn`, `/api/sqm`,
`/api/guestwifi`, `/api/iotwifi`, `/api/portforward`, `/api/netifyd`,
`/api/nftqos`) con forma
`{ "state": ..., "rolled_back": ..., "status": "applied|rolled_back|failed" }`.
Toda escritura exige cookie de sesión.

</details>

## Qué viene

Reciente: análisis de tráfico por aplicación con timeline, límites de ancho
de banda por dispositivo sobre nftables y soporte mipsle para hardware
antiguo. Lo siguiente: un feed de paquetes propio para que owut/ASU conserve
NetGrip dentro de tu imagen de firmware
([#63](https://github.com/gnacho/netgrip/issues/63)) y mantener la demo
pública al día del panel. Las ideas y reportes en los
[issues](https://github.com/gnacho/netgrip/issues) dirigen qué se construye.

## Desarrollo

Stack: Go (binario estático único) + React 19 + TypeScript + Vite + Tailwind,
embebido con `go:embed`. Sin base de datos externa.

```sh
cd app && npm ci
npm run dev      # dev server del frontend (el proxy /api está en vite.config.ts)
go build -o netgrip ./cmd/netgrip
go test ./...
```

El CI compila el frontend, cross-compila y empaqueta `.apk`/`.ipk` con el
SDK de OpenWrt en cada tag de release.

## Licencia

AGPL-3.0-only. Ver [LICENSE](LICENSE).

Construido por gnacho como proyecto personal self-hosted. Si NetGrip te
sirve, una estrella en GitHub es la forma de dar las gracias; issues y PRs
son bienvenidos.
