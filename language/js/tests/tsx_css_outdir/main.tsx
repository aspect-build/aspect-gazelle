import "./styles.css?no-inline"
import "./styles.css#no-inline"
import "./styles.css"
import "./sub/lib"

export const App = () => (
  <div>
    <img src="./images/logo.webp" />
  </div>
)
export const logo = new URL("./logo.png", import.meta.url)
export const logo2 = new URL("logo.png#2", import.meta.url)
