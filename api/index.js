import puppeteer from 'puppeteer-extra';
import StealthPlugin from 'puppeteer-extra-plugin-stealth';

puppeteer.use(StealthPlugin());

export default async function handler(req, res) {
    res.setHeader('Access-Control-Allow-Origin', '*');
    res.setHeader('Content-Type', 'text/plain; charset=utf-8');

    const rawPath = req.url.replace(/^\/|\/$/g, '').split('?')[0];
    
    if (!rawPath || rawPath === 'api') {
        return res.status(400).send("Error: Provide the file ID or URL in the path");
    }

    const parts = rawPath.split('/');
    const fileID = parts[parts.length - 1];

    if (!fileID) {
        return res.status(400).send("Error: Could not extract file ID");
    }

    const domain = rawPath.includes('buzzheavier.com') ? 'buzzheavier.com' : 'bzzhr.to';
    const baseURL = `https://${domain}/${fileID}`;

    let browser = null;

    try {
        const chromium = (await import('@sparticuz/chromium')).default;

        browser = await puppeteer.launch({
            args: chromium.args,
            defaultViewport: chromium.defaultViewport,
            executablePath: await chromium.executablePath(),
            headless: chromium.headless,
            ignoreHTTPSErrors: true,
        });

        const page = await browser.newPage();

        await page.goto(baseURL, { waitUntil: 'domcontentloaded', timeout: 8000 });
        await page.waitForSelector('.copy', { timeout: 8000 });

        const finalUrl = await page.evaluate(async (url) => {
            const html = document.body.innerHTML;
            
            const match = html.match(/\?t=([A-Za-z0-9._-]+)/);
            if (!match) return null;

            const downloadUrl = `${url}/download?t=${match[1]}`;

            const response = await fetch(downloadUrl, {
                headers: {
                    'hx-request': 'true',
                    'referer': url,
                    'accept': '*/*'
                }
            });

            return response.url;
        }, baseURL);

        if (!finalUrl) {
            return res.status(404).send("Error: Security token not found on page");
        }

        res.setHeader('Cache-Control', 's-maxage=345600, stale-while-revalidate');
        return res.status(200).send(finalUrl);

    } catch (error) {
        if (error.name === 'TimeoutError') {
            return res.status(504).send("Error: Cloudflare Turnstile verification timed out. Try again.");
        }
        return res.status(500).send(`Error: Server failure - ${error.message}`);
    } finally {
        if (browser !== null) {
            await browser.close();
        }
    }
}
