/*!
 * Color mode toggler with OS change detection
 */

(() => {
	'use strict';

	const STORAGE_KEY = 'theme_preference';

	const getSystemTheme = () => {
		if (window.matchMedia('(prefers-color-scheme: dark)').matches) {
			return 'dark';
		}
		return 'light';
	};

	const getStoredTheme = () => {
		const stored = localStorage.getItem(STORAGE_KEY);
		if (stored === 'light' || stored === 'dark' || stored === 'system') {
			return stored;
		}
		return 'system';
	};

	const getEffectiveTheme = () => {
		const preference = getStoredTheme();
		if (preference === 'system') {
			return getSystemTheme();
		}
		return preference;
	};

	window.applyTheme = () => {
		const theme = getEffectiveTheme();
		if (theme === 'dark') {
			document.documentElement.classList.add('dark');
		} else {
			document.documentElement.classList.remove('dark');
		}
	};

	window.updateThemePreference = (preference) => {
		localStorage.setItem(STORAGE_KEY, preference);
		window.applyTheme();
	};

	window.getThemePreference = () => {
		return getStoredTheme();
	};

	applyTheme();

	window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
		if (getStoredTheme() === 'system') {
			applyTheme();
		}
	});
})();