import "./styles.css?no-inline"
import "./styles.css#no-inline"
import "./styles.css"

export const App = () => (
  <div>
    <img src="./images/logo.png" />
  </div>
)

export const logo = new URL("logo.png", import.meta.url)
export const logo2 = new URL("./logo.png?2", import.meta.url)
export const logo3 = new URL("/logo.png?3", import.meta.url)
