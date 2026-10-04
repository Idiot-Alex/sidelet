// Local diagnostics only; this helper is never linked into the application.
#include <libproc.h>
#include <mach/mach_time.h>
#include <sys/resource.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

// Apple's XNU proc_info_private.h: flavor 20, two coalition IDs + 3 reserves.
// This private diagnostic ABI may change; fail rather than guess attribution.
// https://github.com/apple-oss-distributions/xnu/blob/main/bsd/sys/proc_info_private.h
struct SideletCoalitions { uint64_t ids[2], reserved[3]; };

static void printJSONString(const char *text) {
    putchar('"');
    for (const unsigned char *p=(const unsigned char *)text; *p; p++) {
        if (*p=='"' || *p=='\\') printf("\\%c",*p);
        else if (*p<32) printf("\\u%04x",*p);
        else putchar(*p);
    }
    putchar('"');
}

int main(int argc, char **argv) {
    if (argc != 2) { fprintf(stderr,"usage: macos-process-snapshot root-pid\n"); return 2; }
    int root = atoi(argv[1]);
    if (root <= 0) return 2;
    int bytes = proc_listallpids(NULL, 0) * sizeof(pid_t) + 4096;
    pid_t *pids = calloc(1, bytes);
    if (!pids) return 1;
    int count = proc_listallpids(pids, bytes), first = 1;
    mach_timebase_info_data_t timebase;
    if (mach_timebase_info(&timebase) != KERN_SUCCESS || !timebase.denom) { free(pids); return 1; }
    printf("[");
    for (int i=0; i<count; i++) {
        char path[PROC_PIDPATHINFO_MAXSIZE] = {0};
        if (proc_pidpath(pids[i],path,sizeof(path)) <= 0) continue;
        const char *name = strrchr(path,'/'); name = name ? name+1 : path;
        if (pids[i] == root && strcmp(name,"sidelet-spike") && strcmp(name,"sidelet")) {
            fprintf(stderr,"Root pid %d is not Sidelet\n",root);
            free(pids); return 1;
        }
        if (pids[i] != root && strncmp(name,"com.apple.WebKit.",17)) continue;
        struct SideletCoalitions coal = {0};
        struct rusage_info_v2 usage = {0};
        if (proc_pidinfo(pids[i],20,0,&coal,sizeof(coal)) != sizeof(coal) ||
            proc_pid_rusage(pids[i],RUSAGE_INFO_V2,(rusage_info_t *)&usage) != 0) {
            fprintf(stderr,"Unable to read coalition/resources for pid %d\n",pids[i]);
            free(pids); return 1;
        }
        // The accepted Sidelet/WebKit executable names have no JSON metacharacters.
        printf("%s{\"pid\":%d,\"name\":\"%s\",\"resourceCoalition\":%llu,\"jetsamCoalition\":%llu,\"start\":%llu,\"cpuNanoseconds\":%llu,\"residentBytes\":%llu,\"footprintBytes\":%llu",
            first ? "" : ",",pids[i],name,
            (unsigned long long)coal.ids[0],(unsigned long long)coal.ids[1],
            (unsigned long long)usage.ri_proc_start_abstime,
            (unsigned long long)(((__uint128_t)usage.ri_user_time+usage.ri_system_time)*timebase.numer/timebase.denom),
            (unsigned long long)usage.ri_resident_size,(unsigned long long)usage.ri_phys_footprint);
        printf(",\"executable\":"); printJSONString(path); printf("}");
        first=0;
    }
    printf("]\n"); free(pids); return 0;
}
