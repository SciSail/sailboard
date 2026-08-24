import React from 'react'
import {createRoot} from 'react-dom/client'
import './style.css'
import Settings from './Settings'

// See main.tsx's identical listener for why: a DOM-level backstop for the WebView2-level default
// context menu suppression (options.App.EnableDefaultContextMenu), which isn't reliable on every
// device.
document.addEventListener('contextmenu', event => event.preventDefault())

const container = document.getElementById('root')

const root = createRoot(container!)

root.render(
    <React.StrictMode>
        <Settings/>
    </React.StrictMode>
)
