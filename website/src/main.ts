import { mount } from 'svelte';
import App from './App.svelte';
import './demo/theme.css';
import './site.css';

mount(App, { target: document.getElementById('app')! });
