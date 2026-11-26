import type { Theme } from 'vitepress';
import DefaultTheme from 'vitepress/theme';
import { inBrowser } from 'vitepress';
import { nextTick } from 'vue';
import "@catppuccin/vitepress/theme/frappe/blue.css";

import './custom.css';
import './exhibit.css';
import { initCharts } from './charts';

export default {
	extends: DefaultTheme,
	enhanceApp({ router }) {
		if (!inBrowser) return;

		// the exhibits' charts get their hover once the page is in the DOM — on first
		// load, and after every client-side navigation
		const init = () => nextTick(() => initCharts());
		router.onAfterPageLoad = init;
		router.onAfterRouteChange = init;
	},
} satisfies Theme;
