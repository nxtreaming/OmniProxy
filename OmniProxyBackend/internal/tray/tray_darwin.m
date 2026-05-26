#import <Cocoa/Cocoa.h>
#import <dispatch/dispatch.h>
#include <stdlib.h>

extern char* omniproxyTrayDarwinStatusLabel(void);
extern char* omniproxyTrayDarwinPortLabel(void);
extern int omniproxyTrayDarwinProxyRunning(void);
extern void omniproxyTrayDarwinToggleProxy(void);
extern void omniproxyTrayDarwinShowWindow(void);
extern void omniproxyTrayDarwinQuit(void);

@interface OmniProxyStatusDelegate : NSObject<NSMenuDelegate>
@property(nonatomic, strong) NSMenuItem *statusItem;
@property(nonatomic, strong) NSMenuItem *portItem;
@property(nonatomic, strong) NSMenuItem *toggleItem;
@end

static NSStatusItem *omniproxyStatusItem = nil;
static OmniProxyStatusDelegate *omniproxyStatusDelegate = nil;

static NSString* omniStringFromCString(char *value) {
    if (value == NULL) {
        return @"";
    }
    NSString *result = [NSString stringWithUTF8String:value];
    free(value);
    return result == nil ? @"" : result;
}

static void omniproxyDarwinRefreshStatusMenu(void) {
    if (omniproxyStatusDelegate == nil) {
        return;
    }
    NSString *status = omniStringFromCString(omniproxyTrayDarwinStatusLabel());
    NSString *port = omniStringFromCString(omniproxyTrayDarwinPortLabel());
    BOOL running = omniproxyTrayDarwinProxyRunning() != 0;
    [omniproxyStatusDelegate.statusItem setTitle:status];
    [omniproxyStatusDelegate.portItem setTitle:port];
    [omniproxyStatusDelegate.toggleItem setTitle:(running ? @"停止代理" : @"启动代理")];
}

@implementation OmniProxyStatusDelegate
- (void)menuWillOpen:(NSMenu *)menu {
    omniproxyDarwinRefreshStatusMenu();
}
- (void)toggleProxy:(id)sender {
    omniproxyTrayDarwinToggleProxy();
}
- (void)showWindow:(id)sender {
    omniproxyTrayDarwinShowWindow();
}
- (void)quit:(id)sender {
    omniproxyTrayDarwinQuit();
}
@end

void omniproxyDarwinStartStatusItem(const char *tooltip) {
    NSString *tip = tooltip == NULL ? @"OmniProxy" : [NSString stringWithUTF8String:tooltip];
    dispatch_async(dispatch_get_main_queue(), ^{
        if (omniproxyStatusItem != nil) {
            return;
        }
        omniproxyStatusDelegate = [OmniProxyStatusDelegate new];
        omniproxyStatusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSSquareStatusItemLength];
        NSButton *button = [omniproxyStatusItem button];
        if (button != nil) {
            [button setTitle:@"OP"];
            [button setToolTip:tip == nil ? @"OmniProxy" : tip];
        }

        NSMenu *menu = [[NSMenu alloc] initWithTitle:@"OmniProxy"];
        [menu setDelegate:omniproxyStatusDelegate];

        omniproxyStatusDelegate.statusItem = [[NSMenuItem alloc] initWithTitle:@"代理状态" action:nil keyEquivalent:@""];
        [omniproxyStatusDelegate.statusItem setEnabled:NO];
        [menu addItem:omniproxyStatusDelegate.statusItem];

        omniproxyStatusDelegate.portItem = [[NSMenuItem alloc] initWithTitle:@"端口" action:nil keyEquivalent:@""];
        [omniproxyStatusDelegate.portItem setEnabled:NO];
        [menu addItem:omniproxyStatusDelegate.portItem];

        [menu addItem:[NSMenuItem separatorItem]];

        omniproxyStatusDelegate.toggleItem = [[NSMenuItem alloc] initWithTitle:@"启动代理" action:@selector(toggleProxy:) keyEquivalent:@""];
        [omniproxyStatusDelegate.toggleItem setTarget:omniproxyStatusDelegate];
        [menu addItem:omniproxyStatusDelegate.toggleItem];

        NSMenuItem *showItem = [[NSMenuItem alloc] initWithTitle:@"打开主界面" action:@selector(showWindow:) keyEquivalent:@""];
        [showItem setTarget:omniproxyStatusDelegate];
        [menu addItem:showItem];

        [menu addItem:[NSMenuItem separatorItem]];

        NSMenuItem *quitItem = [[NSMenuItem alloc] initWithTitle:@"退出 OmniProxy" action:@selector(quit:) keyEquivalent:@""];
        [quitItem setTarget:omniproxyStatusDelegate];
        [menu addItem:quitItem];

        [omniproxyStatusItem setMenu:menu];
        omniproxyDarwinRefreshStatusMenu();
    });
}

void omniproxyDarwinStopStatusItem(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (omniproxyStatusItem != nil) {
            [[NSStatusBar systemStatusBar] removeStatusItem:omniproxyStatusItem];
            omniproxyStatusItem = nil;
            omniproxyStatusDelegate = nil;
        }
    });
}
