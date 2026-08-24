import React from 'react'
import {createRoot} from 'react-dom/client'
import './style.css'
import App from './App'

// main.go's production build already asks WebView2 to suppress its own default right-click menu
// (options.App.EnableDefaultContextMenu is left false, and only f.debug — a `wails dev`/-debug
// build — ORs it back on), but that WebView2-level setting has been seen to not reliably hold on
// every WebView2 Runtime version/device combo, and this app has no use for a raw browser context
// menu (Back/Reload/Inspect) anyway. A second, DOM-level backstop costs nothing and closes the
// gap on whatever device(s) the WebView2-level suppression doesn't cover.
document.addEventListener('contextmenu', event => event.preventDefault())

const container = document.getElementById('root')

const root = createRoot(container!)

root.render(
    <React.StrictMode>
        <App/>
    </React.StrictMode>
)
