import { mount } from 'svelte';
// Design system global dimuat sekali di entry, sebelum komponen apa pun
import './styles/index.css';
import App from './App.svelte';

const target = document.getElementById('app');
if (!target) {
  throw new Error('Elemen #app tidak ditemukan di index.html');
}

const app = mount(App, { target });

export default app;
