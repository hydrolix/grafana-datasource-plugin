## Local development setup

### Prerequisites

Make sure you have the following dependencies installed first:

- [Git](https://git-scm.com/)
- [Go](https://golang.org/dl/) (see [go.mod](go.mod#L3) for minimum required version)
- [Mage](https://magefile.org/)
- [Node.js](https://nodejs.org) (LTS version recommended)

### Running the plugin locally

Follow these steps to set up and run the plugin in your local environment:

1. Clone this repository into your local environment. The frontend code lives in the [src](src) folder,
   alongside the [plugin.json](src/plugin.json) file. The backend Go code is in the [pkg](pkg) folder.
2. Go to the directory where you cloned this repository:
   ```shell
   cd <grafana-datasource-plugin>
   ```
3. Install the dependencies:
   ```shell
   npm install
   ```
4. Build the plugin frontend:
   ```shell
   npm run dev
   ```
5. In a new terminal window, build the plugin backend:
   ```shell
   mage -v build:linux
   ```
6. Start Grafana:
   ```shell
   docker compose up
   ```
7. Open Grafana at http://localhost:3000, and then go to **Administration** > **Plugins**. Make sure that
   this plugin is there.


