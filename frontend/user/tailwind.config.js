/** @type {import('tailwindcss').Config} */
export default {
    content: [
        "./index.html",
        "./src/**/*.{vue,js,ts,jsx,tsx}",
    ],
    darkMode: 'class',
    theme: {
        extend: {
            colors: {
                // Phase 9 品牌色：实际 token 在 style.css 的 @theme inline 中定义为 CSS variable，
                // 这里仅作登记；运行时由 /public/config 的 brand.primary_color 注入 --brand-primary。
                brand: {
                    primary: 'var(--brand-primary)',
                    light: 'var(--brand-primary-light)',
                },
            },
        },
    },
    plugins: [],
}

