import { ref, computed, reactive } from "vue";
import { defineStore } from "pinia";
import { services } from '../../wailsjs/go/models'
import { setThemeColor } from '../utils/utils'

export const themeStore = defineStore("themeStore",  {
    state:() =>{
        return {
            // 主题模式：'light' | 'dark'
            theme: 'light' as 'light' | 'dark',
            // 主题颜色
            color: '#ff6b00',
        }
    },
    getters: {
        // 是否为暗色主题
        isDark: (state) => state.theme === 'dark',
        // 当前主题类名
        themeClass: (state) => `theme-${state.theme}`,
    },
    actions: {
        // 切换主题
        toggleTheme() {
            this.theme = this.theme === 'light' ? 'dark' : 'light';
            this.applyTheme();
        },
        // 设置主题
        setTheme(theme: 'light' | 'dark') {
            this.theme = theme;
            this.applyTheme();
        },
        // 设置主题色
        setThemeColor(color: string) {
            this.color = color;
            setThemeColor(color);
        },
        // 应用主题到 DOM
        applyTheme() {
            const html = document.documentElement;
            // 移除之前的主题类
            html.classList.remove('theme-light', 'theme-dark');
            // 添加当前主题类
            html.classList.add(this.themeClass);
            // 设置 CSS 变量
            html.setAttribute('data-theme', this.theme);
            // 应用主题色
            setThemeColor(this.color);
        },
        // 初始化主题
        initTheme() {
            this.applyTheme();
        }
    },
    persist: true,
});
