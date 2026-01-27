import "./styles.css?no-inline"
import "./styles.css#no-inline"
import "./styles.css"

export const App = () => (
  <div>
    <img src="./logo.png" />
    <img src="./styles.css" />
  </div>
)

export const logo = new URL("logo.png", import.meta.url)
export const styles = new URL("styles.css", import.meta.url)
