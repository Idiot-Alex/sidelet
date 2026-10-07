import { mount } from 'svelte';
import { connect } from '../../src/lib/bridge';
import DragFixture from './LabelDragFixture.svelte';
import '../../src/theme.css';

// Load the real bridge subscriptions first. The fixture component supplies an
// isolated in-memory host; it never connects to native windows or user data.
await connect(() => {}, () => {}, () => {}, () => {});
mount(DragFixture, { target: document.getElementById('app')! });
