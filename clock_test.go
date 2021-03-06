package clock

import (
	"math"
	"math/rand"
	"sync"
	"testing"
	"time"
)

var (
	r = rand.New(rand.NewSource(time.Now().Unix()))
)

//_Counter 支持并发的计数器
type _Counter struct {
	sync.Mutex
	counter int
}

func (counter *_Counter) AddOne() {
	counter.Lock()
	counter.counter++
	counter.Unlock()
}
func (counter *_Counter) Count() int {
	return counter.counter
}

func TestClock_Create(t *testing.T) {
	myClock := NewClock()
	if myClock.WaitJobs() != 0 || myClock.Count() != 0 {
		t.Errorf("JobList init have error.len=%d,count=%d", myClock.WaitJobs(), myClock.Count())
		//joblist.Debug()
	}

}

func TestClock_AddOnceJob(t *testing.T) {
	var (
		randscope = 50 * 1000 * 1000 //随机范围
		interval  = time.Millisecond*100 + time.Duration(r.Intn(randscope))
		myClock   = NewClock()
		jobFunc   = func() {
			//fmt.Println("任务事件")
		}
	)

	//插入时间间隔≤0，应该不允许添加
	if _, inserted := myClock.AddJobWithTimeout(0, jobFunc); inserted {
		t.Error("任务添加失败，加入了间隔时间≤0的任务。")
	}

	if _, inserted := myClock.AddJobWithTimeout(interval, jobFunc); !inserted {
		t.Error("任务添加失败，未加入任务。")
	}

	time.Sleep(time.Second)

	if myClock.Count() != 1 {
		t.Errorf("任务执行存在问题，应该执行%d次,实际执行%d次", 1, myClock.Count())
	}
}

//TestClock_WaitJobs 测试当前待执行任务列表中的事件
func TestClock_WaitJobs(t *testing.T) {
	var (
		myClock   = NewClock()
		randscope = 50 * 1000 * 1000 //随机范围
		interval  = time.Millisecond*50 + time.Duration(r.Intn(randscope))
		jobFunc   = func() {
			//fmt.Println("任务事件")
		}
	)
	job, inserted := myClock.AddJobRepeat(interval, 0, jobFunc)
	if !inserted {
		t.Error("定时任务创建失败")
	}
	time.Sleep(time.Second)

	if myClock.WaitJobs() != 1 {
		t.Error("任务添加异常")
	}
	if myClock.WaitJobs() != 1 {
		t.Error("数据列表操作获取的数据与Clock实际情况不一致！")
	}
	myClock.DelJob(job)

}

//TestClock_AddRepeatJob 测试重复任务定时执行情况
func TestClock_AddRepeatJob(t *testing.T) {
	var (
		myClock   = NewClock()
		jobsNum   = uint64(1000)                                            //执行次数
		randscope = 50 * 1000                                               //随机范围
		interval  = time.Microsecond*100 + time.Duration(r.Intn(randscope)) //100-150µs时间间隔
		counter   = new(_Counter)
	)
	f := func() {
		counter.AddOne()
	}
	job, inserted := myClock.AddJobRepeat(interval, jobsNum, f)
	if !inserted {
		t.Error("任务初始化失败，任务事件没有添加成功")
	}
	for range job.C() {

	}
	//重复任务的方法是协程调用，可能还没有执行，job.C就已经退出，需要阻塞观察
	time.Sleep(time.Second)
	if int(myClock.Count()) != counter.Count() || counter.Count() != int(jobsNum) {
		t.Errorf("任务添加存在问题,应该%v次，实际执行%v\n", jobsNum, counter.Count())
	}

}

//TestClock_AddRepeatJob2 测试间隔时间不同的两个重复任务，是否会交错执行
func TestClock_AddRepeatJob2(t *testing.T) {
	var (
		myClock    = NewClock()
		interval1  = time.Millisecond * 20 //间隔20毫秒
		interval2  = time.Millisecond * 20 //间隔20毫秒
		singalChan = make(chan int, 10)
	)
	jobFunc := func(sigal int) {
		singalChan <- sigal

	}
	go func() {
		cacheSigal := 2
		for z := range singalChan {
			if z == cacheSigal {
				t.Error("两个任务没有间隔执行")
			} else {
				cacheSigal = z
			}
		}
	}()
	event1, inserted1 := myClock.AddJobRepeat(interval1, 0, func() { jobFunc(1) })
	time.Sleep(time.Millisecond * 10)
	event2, inserted2 := myClock.AddJobRepeat(interval2, 0, func() { jobFunc(2) })

	if !inserted1 || !inserted2 {
		t.Error("任务初始化失败，没有添加成功")
	}
	time.Sleep(time.Second)

	myClock.DelJob(event1)
	myClock.DelJob(event2)

}

//TestClock_AddMixJob 测试一次性任务+重复性任务的运行撤销情况
func TestClock_AddMixJob(t *testing.T) {
	var (
		myClock  = NewClock()
		counter1 int
		counter2 int
	)
	f1 := func() {
		counter1++
	}
	f2 := func() {
		counter2++
	}
	_, inserted1 := myClock.AddJobWithTimeout(time.Millisecond*500, f1)
	_, inserted2 := myClock.AddJobRepeat(time.Millisecond*300, 0, f2)

	if !inserted1 && !inserted2 {
		t.Fatal("任务添加失败！")
	}
	time.Sleep(time.Second * 2)
	if counter1 != 1 || counter2 < 5 {
		t.Errorf("执行次数异常！,一次性任务执行了:%v，重复性任务执行了%v\n", counter1, counter2)
	}
}

//TestClock_AddJobs 测试短时间，高频率的情况下，事件提醒功能能否实现。
func TestClock_AddJobs(t *testing.T) {
	var (
		jobsNum   = 200000                 //添加任务数量
		randscope = 1 * 1000 * 1000 * 1000 //随机范围1秒
		myClock   = NewClock()
		counter   = &_Counter{}
		wg        sync.WaitGroup
	)
	f := func() {
		//schedule nothing
	}
	//创建jobsNum个任务，每个任务都会间隔[1,2)秒内执行一次
	for i := 0; i < jobsNum; i++ {
		job, inserted := myClock.AddJobWithTimeout(time.Second+time.Duration(r.Intn(randscope)), f)
		if !inserted {
			t.Error("任务添加存在问题")
			break
		}
		wg.Add(1)
		go func() {
			<-job.C()
			counter.AddOne() //收到消息就计数
			wg.Done()
		}()
	}
	wg.Wait()
	if jobsNum != int(myClock.Count()) || jobsNum != counter.Count() {
		t.Errorf("应该执行%v次，实际执行%v次,外部信号接受到%v次。\n", jobsNum, myClock.Count(), counter.Count())
	}
}

//TestClock_Delay_200kJob 测试20万条任务下，其中任意一条数据从加入到执行的时间延迟，是否超过约定的最大值
// 目标：
//	1.不得有任何一条事件提醒，延时超过2s，即平均延时在10µs内。
// Note:笔记本(尤其是windows操作系统）,云服务可能无法通过测试
func TestClock_Delay_200kJob(t *testing.T) {
	t.Skip()
	var (
		jobsNum     = 200000 //添加任务数量
		myClock     = NewClock()
		jobInterval = time.Second
		mut         sync.Mutex
		maxDelay    int64
	)
	start := time.Now().Add(time.Second)

	fn := func() {
		delay := time.Now().Sub(start).Nanoseconds()
		if delay > maxDelay {
			mut.Lock()
			maxDelay = delay
			mut.Unlock()
		}
	}

	//初始化20万条任务。考虑到初始化耗时，延时1秒后启动
	for i := 0; i < jobsNum; i++ {
		myClock.AddJobWithTimeout(jobInterval, fn)

	}
	time.Sleep(time.Second * 3)
	if jobsNum != int(myClock.Count()) {
		t.Errorf("应该执行%v次，实际执行%v次。所有值应该相等。\n", jobsNum, myClock.Count())
	}
	if maxDelay > (time.Second * 2).Nanoseconds() {
		t.Errorf("超过了允许的最大时间%v秒，实际耗时:%v ms\n", time.Second*2, maxDelay/1e6)
	}
	//t.Logf("实际耗时:%vms \n", maxDelay/1e6)

}

// test miniheap ,compare performance
//func TestClock_Delay_100kJob1(t *testing.T) {
//	var (
//		jobsNum     = 100000 //添加任务数量
//		myClock     = NewTimer()
//		jobInterval = time.Second
//		mut         sync.Mutex
//		maxDelay    int64
//	)
//	start := time.Now().Add(time.Second)
//	fn := func() {
//		delay := time.Now().Sub(start).Nanoseconds()
//		if delay > maxDelay {
//			mut.Lock()
//			maxDelay = delay
//			mut.Unlock()
//		}
//	}
//	//初始化20万条任务。考虑到初始化耗时，延时1秒后启动
//	for i := 0; i < jobsNum; i++ {
//		myClock.NewItem(jobInterval, fn)
//
//	}
//	time.Sleep(time.Second * 2)
//	if maxDelay > (time.Second * 2).Nanoseconds() {
//		t.Errorf("超过了允许的最大时间%v秒，实际耗时:%v ms\n", time.Second*2, maxDelay/1e6)
//	}
//	t.Logf("实际耗时:%vms \n", maxDelay/1e6)
//
//}

//TestClock_DelJob 检测待运行任务中，能否随机删除一条任务。
func TestClock_DelJob(t *testing.T) {
	//思路：
	//新增一定数量的任务，延时1秒开始执行
	//在一秒内，删除所有的任务。
	//如果执行次数=0，说明一秒内无法满足对应条数的增删
	var (
		jobsNum   = 20000
		randscope = 1 * 1000 * 1000 * 1000
		jobs      = make([]Job, jobsNum)
		delmod    = r.Intn(jobsNum)
		myClock   = NewClock()
	)
	for i := 0; i < jobsNum; i++ {
		delay := time.Second + time.Duration(r.Intn(randscope)) //增加一秒作为延迟，以避免删除的时候，已经存在任务被通知执行，导致后续判断失误
		job, _ := myClock.AddJobWithTimeout(delay, nil)
		jobs[i] = job
	}

	deleted := myClock.DelJob(jobs[delmod])
	if !deleted || myClock.WaitJobs() != uint(jobsNum-1) {
		t.Errorf("任务删除%v，删除后，应该只剩下%v条任务，实际还有%v条\n", deleted, myClock.Count(), jobsNum-1)

	}
}

//TestClock_DelJobs 本测试主要检测添加、删除任务的性能。保证每秒1万次新增+删除操作。
func TestClock_DelJobs(t *testing.T) {
	//思路：
	//新增一定数量的任务，延时1秒开始执行
	//在一秒内，删除所有的任务。
	//如果执行次数！=0，说明一秒内无法满足对应条数的增删
	var (
		myClock     = NewClock()
		jobsNum     = 20000
		randscope   = 1 * 1000 * 1000 * 1000
		jobs        = make([]Job, jobsNum)
		wantdeljobs = make([]Job, jobsNum)
	)
	for i := 0; i < jobsNum; i++ {
		delay := time.Second + time.Duration(r.Intn(randscope)) //增加一秒作为延迟，以避免删除的时候，已经存在任务被通知执行，导致后续判断失误
		job, _ := myClock.AddJobWithTimeout(delay, nil)
		jobs[i] = job
		wantdeljobs[i] = job
	}

	myClock.DelJobs(wantdeljobs)

	if 0 != int(myClock.Count()) || myClock.WaitJobs() != 0 || myClock.jobList.Len() != 0 {
		t.Errorf("应该执行%v次，实际执行%v次,此时任务队列中残余记录,myClock.actionindex.len=%v,jobList.len=%v\n", jobsNum-len(wantdeljobs), myClock.Count(), myClock.WaitJobs(), myClock.jobList.Len())

	}
}

func BenchmarkClock_AddJob(b *testing.B) {
	myClock := NewClock()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		newjob, inserted := myClock.AddJobWithTimeout(time.Millisecond*5, nil)
		if !inserted {
			b.Error("can not insert jobItem")
			break
		}
		<-newjob.C()
	}
}

// 测试通道消息传送的时间消耗
func BenchmarkChan(b *testing.B) {
	tmpChan := make(chan time.Duration, 1)
	maxnum := int64(math.MaxInt64)
	for i := 0; i < b.N; i++ {
		dur := time.Duration(maxnum - time.Now().UnixNano())
		tmpChan <- dur
		<-tmpChan

	}
}
