// Social media content scraper example
// This example shows how to scrape social media posts and interactions
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/tmc/misc/chrome-to-har/internal/recorder"
)

type SocialPost struct {
	Author      string    `json:"author"`
	Content     string    `json:"content"`
	Timestamp   string    `json:"timestamp,omitempty"`
	Likes       int       `json:"likes,omitempty"`
	Comments    int       `json:"comments,omitempty"`
	Shares      int       `json:"shares,omitempty"`
	URL         string    `json:"url,omitempty"`
	ImageURLs   []string  `json:"image_urls,omitempty"`
	Hashtags    []string  `json:"hashtags,omitempty"`
	Mentions    []string  `json:"mentions,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run social-media-scraper.go <social-media-url>")
		fmt.Println("Example: go run social-media-scraper.go 'https://twitter.com/search?q=golang'")
		os.Exit(1)
	}

	url := os.Args[1]

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Create Chrome browser with social media friendly options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"),
		chromedp.WindowSize(1920, 1080),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-extensions", false),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	chromeCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Create recorder
	rec := recorder.New()

	var posts []SocialPost

	// Navigate and scrape
	err := chromedp.Run(chromeCtx,
		rec.Start(),
		chromedp.Navigate(url),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(5*time.Second), // Wait for dynamic content
		
		// Scroll to load more posts
		chromedp.Evaluate(`
			for (let i = 0; i < 3; i++) {
				window.scrollTo(0, document.body.scrollHeight);
				await new Promise(resolve => setTimeout(resolve, 2000));
			}
		`, nil),
		chromedp.Sleep(3*time.Second),
		
		// Extract posts
		chromedp.Evaluate(`
			(function() {
				const posts = [];
				
				// Common social media post selectors
				const selectors = [
					'[data-testid="tweet"]',
					'[data-testid="post"]',
					'article',
					'.post',
					'[class*="post"]',
					'[class*="tweet"]',
					'[class*="status"]',
					'[role="article"]'
				];
				
				let elements = [];
				for (const selector of selectors) {
					elements = document.querySelectorAll(selector);
					if (elements.length > 0) break;
				}
				
				console.log('Found', elements.length, 'post elements');
				
				elements.forEach(el => {
					// Extract author
					let author = '';
					const authorSelectors = [
						'[data-testid="User-Name"]',
						'[data-testid="User-Names"]',
						'.username',
						'[class*="username"]',
						'[class*="author"]',
						'[class*="user"]',
						'h3',
						'strong'
					];
					
					for (const sel of authorSelectors) {
						const authorEl = el.querySelector(sel);
						if (authorEl && authorEl.textContent.trim()) {
							author = authorEl.textContent.trim();
							break;
						}
					}
					
					// Extract content
					let content = '';
					const contentSelectors = [
						'[data-testid="tweetText"]',
						'[data-testid="post-content"]',
						'.tweet-text',
						'.post-content',
						'[class*="content"]',
						'[class*="text"]',
						'p'
					];
					
					for (const sel of contentSelectors) {
						const contentEl = el.querySelector(sel);
						if (contentEl && contentEl.textContent.trim()) {
							content = contentEl.textContent.trim();
							break;
						}
					}
					
					// Extract timestamp
					let timestamp = '';
					const timeSelectors = [
						'time',
						'[data-testid="Time"]',
						'[class*="time"]',
						'[class*="date"]',
						'[datetime]'
					];
					
					for (const sel of timeSelectors) {
						const timeEl = el.querySelector(sel);
						if (timeEl) {
							timestamp = timeEl.getAttribute('datetime') || timeEl.textContent.trim();
							break;
						}
					}
					
					// Extract engagement metrics
					let likes = 0, comments = 0, shares = 0;
					const metricsElements = el.querySelectorAll('[role="button"], [data-testid*="like"], [data-testid*="retweet"], [data-testid*="reply"]');
					
					metricsElements.forEach(metricEl => {
						const text = metricEl.textContent.trim();
						const number = parseInt(text.replace(/[^\d]/g, '')) || 0;
						
						if (text.includes('like') || metricEl.getAttribute('data-testid')?.includes('like')) {
							likes = number;
						} else if (text.includes('comment') || text.includes('reply') || metricEl.getAttribute('data-testid')?.includes('reply')) {
							comments = number;
						} else if (text.includes('share') || text.includes('retweet') || metricEl.getAttribute('data-testid')?.includes('retweet')) {
							shares = number;
						}
					});
					
					// Extract hashtags and mentions
					const hashtags = [];
					const mentions = [];
					
					if (content) {
						const hashtagMatches = content.match(/#\w+/g);
						if (hashtagMatches) {
							hashtags.push(...hashtagMatches);
						}
						
						const mentionMatches = content.match(/@\w+/g);
						if (mentionMatches) {
							mentions.push(...mentionMatches);
						}
					}
					
					// Extract images
					const imageURLs = [];
					const images = el.querySelectorAll('img');
					images.forEach(img => {
						const src = img.src || img.dataset.src;
						if (src && !src.includes('avatar') && !src.includes('profile')) {
							imageURLs.push(src);
						}
					});
					
					// Extract post URL
					let postUrl = '';
					const linkEl = el.querySelector('a[href*="/status/"], a[href*="/post/"], a[href*="/p/"]');
					if (linkEl) {
						postUrl = linkEl.href;
					}
					
					if (author && content) {
						posts.push({
							author: author,
							content: content,
							timestamp: timestamp,
							likes: likes,
							comments: comments,
							shares: shares,
							url: postUrl,
							image_urls: imageURLs,
							hashtags: hashtags,
							mentions: mentions
						});
					}
				});
				
				return posts;
			})()
		`, &posts),
		
		rec.Stop(),
	)

	if err != nil {
		log.Fatal(err)
	}

	// Output results
	fmt.Printf("Found %d posts from %s\n\n", len(posts), url)
	
	for i, post := range posts {
		fmt.Printf("Post %d:\n", i+1)
		fmt.Printf("Author: %s\n", post.Author)
		fmt.Printf("Content: %s\n", truncateString(post.Content, 200))
		if post.Timestamp != "" {
			fmt.Printf("Timestamp: %s\n", post.Timestamp)
		}
		if post.Likes > 0 || post.Comments > 0 || post.Shares > 0 {
			fmt.Printf("Engagement: %d likes, %d comments, %d shares\n", post.Likes, post.Comments, post.Shares)
		}
		if len(post.Hashtags) > 0 {
			fmt.Printf("Hashtags: %s\n", strings.Join(post.Hashtags, ", "))
		}
		if len(post.Mentions) > 0 {
			fmt.Printf("Mentions: %s\n", strings.Join(post.Mentions, ", "))
		}
		fmt.Println(strings.Repeat("-", 50))
	}

	// Save as JSON
	jsonData, err := json.MarshalIndent(posts, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	err = os.WriteFile("social-posts.json", jsonData, 0644)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nPosts saved to social-posts.json\n")

	// Save HAR file for analysis
	harData, err := rec.HAR()
	if err == nil {
		err = os.WriteFile("social-scraping.har", []byte(harData), 0644)
		if err == nil {
			fmt.Printf("Network traffic saved to social-scraping.har\n")
		}
	}
}

func truncateString(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length] + "..."
}